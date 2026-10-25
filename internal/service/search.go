package service

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Brands are the catalog's brands, as stored in items.brand.
var Brands = []string{"Hot Wheels", "Mini GT", "Inno64", "Matchbox", "Tomica", "Tarmac Works", "M2 Machines"}

// Categories mirrors the items.category CHECK constraint.
var Categories = []string{"diecast", "diorama"}

// Conditions are the diecast packaging conditions, DioramaConditions the
// diorama ones; together they mirror the items_category_fields CHECK.
var (
	Conditions        = []string{"mint_on_card", "sealed_box", "acrylic_case", "loose_mint", "damaged_packaging"}
	DioramaConditions = []string{"new", "used"}
)

// Sorts are the accepted Filter.Sort values; empty means ending soonest first.
var Sorts = map[string]string{
	"":            "a.end_time ASC",
	"ending_soon": "a.end_time ASC",
	"newest":      "a.start_time DESC",
	"price_asc":   currentPrice + " ASC",
	"price_desc":  currentPrice + " DESC",
}

// currentPrice is what a buyer would pay now: the leading bid, or the start price before any bid.
const currentPrice = "coalesce(a.current_highest_bid, a.start_price)"

const searchLimit = 50

// Filter is a catalog search. It may come from an LLM, so it is untrusted:
// Normalize it, and it only ever reaches SQL as bind parameters.
type Filter struct {
	Category  string   `json:"category"`
	Brand     string   `json:"brand"`
	Series    string   `json:"series"`
	Color     string   `json:"color"`
	Tags      []string `json:"tags"`
	Condition string   `json:"condition"`
	MinPrice  int64    `json:"min_price"`
	MaxPrice  int64    `json:"max_price"`
	Text      string   `json:"text"`
	Sort      string   `json:"sort"`
}

type SearchResult struct {
	Filter   Filter    `json:"filter"`
	Relaxed  bool      `json:"relaxed"` // true when strict filters matched nothing and some were dropped
	Auctions []Auction `json:"auctions"`
}

// Normalize drops values outside the allowed sets instead of failing, since
// a hallucinated field should narrow nothing rather than break the search.
func (f Filter) Normalize() Filter {
	short := func(s string) string {
		r := []rune(strings.Join(strings.Fields(strings.ReplaceAll(s, "\x00", "")), " "))
		return strings.ToValidUTF8(string(r[:min(len(r), 100)]), "")
	}
	out := Filter{Series: short(f.Series), Color: strings.ToLower(short(f.Color)), Text: short(f.Text)}
	for _, b := range Brands {
		if strings.EqualFold(b, strings.TrimSpace(f.Brand)) {
			out.Brand = b
		}
	}
	if c := strings.ToLower(strings.TrimSpace(f.Category)); slices.Contains(Categories, c) {
		out.Category = c
	}
	if slices.Contains(Conditions, f.Condition) || slices.Contains(DioramaConditions, f.Condition) {
		out.Condition = f.Condition
	}
	if _, ok := Sorts[f.Sort]; ok {
		out.Sort = f.Sort
	}
	out.MinPrice, out.MaxPrice = max(f.MinPrice, 0), max(f.MaxPrice, 0)
	if out.MaxPrice > 0 && out.MinPrice > out.MaxPrice {
		out.MaxPrice = 0
	}
	for _, t := range f.Tags {
		t = strings.ToLower(short(t))
		if t != "" && !slices.Contains(out.Tags, t) && len(out.Tags) < 5 {
			out.Tags = append(out.Tags, t)
		}
	}
	return out
}

// Search returns active auctions matching f. With relax, when nothing matches it relaxes
// step by step: first color, series, tags and condition, then price, and
// finally any single text word may match instead of all of them. Brand and
// category always stay, since they carry what the user is looking for.
// Explicit catalog filters pass relax=false: a user who picked a condition
// must not silently get other conditions back.
func (s *Auctions) Search(ctx context.Context, f Filter, relax bool) (SearchResult, error) {
	f = f.Normalize()
	soft := f
	soft.Color, soft.Series, soft.Tags, soft.Condition = "", "", nil, ""
	loose := soft
	loose.MinPrice, loose.MaxPrice = 0, 0

	type try struct {
		f       Filter
		anyWord bool
	}
	tries := []try{{f: f}}
	for _, next := range []Filter{soft, loose} {
		if !relax {
			break
		}
		if !reflect.DeepEqual(next, tries[len(tries)-1].f) {
			tries = append(tries, try{f: next})
		}
	}
	if relax && len(strings.Fields(f.Text)) > 1 {
		tries = append(tries, try{f: loose, anyWord: true})
	}
	for i, t := range tries {
		found, err := s.find(ctx, t.f, t.anyWord)
		if err != nil {
			return SearchResult{}, err
		}
		if len(found) > 0 || i == len(tries)-1 {
			return SearchResult{Filter: f, Relaxed: i > 0, Auctions: found}, nil
		}
	}
	return SearchResult{}, nil // unreachable: tries is never empty
}

// find runs one query. Text words must all match, or with anyWord at least
// one word of 3+ letters must (short words like "di" match almost anything).
func (s *Auctions) find(ctx context.Context, f Filter, anyWord bool) ([]Auction, error) {
	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where = append(where, "a.status = 'active'")
	if f.Category != "" {
		where = append(where, "i.category = "+arg(f.Category))
	}
	if f.Brand != "" {
		where = append(where, "i.brand = "+arg(f.Brand))
	}
	if f.Series != "" {
		where = append(where, "i.series ILIKE '%' || "+arg(escapeLike(f.Series))+" || '%'")
	}
	if f.Color != "" {
		where = append(where, "i.color ILIKE '%' || "+arg(escapeLike(f.Color))+" || '%'")
	}
	if len(f.Tags) > 0 {
		where = append(where, "i.tags @> "+arg(f.Tags))
	}
	if f.Condition != "" {
		where = append(where, "i.condition = "+arg(f.Condition))
	}
	if f.MinPrice > 0 {
		where = append(where, currentPrice+" >= "+arg(f.MinPrice))
	}
	if f.MaxPrice > 0 {
		where = append(where, currentPrice+" <= "+arg(f.MaxPrice))
	}
	// Text compares letters and digits only, so "gtr" finds "GT-R" and "r34" finds "(R34)".
	var words []string
	for _, w := range strings.Fields(f.Text) {
		w = alnum(w)
		if w != "" && (!anyWord || len(w) >= 3) {
			words = append(words, "regexp_replace(lower(concat_ws(' ', i.brand, i.artist, i.model_name, i.series, i.color, "+
				"array_to_string(i.tags, ' '))), '[^a-z0-9]', '', 'g') LIKE '%' || "+arg(w)+" || '%'")
		}
	}
	switch {
	case anyWord && len(words) > 0:
		where = append(where, "("+strings.Join(words, " OR ")+")")
	case anyWord:
		return []Auction{}, nil // only short words: matching on them would return noise
	default:
		where = append(where, words...)
	}

	// The ORDER BY comes from the Sorts allowlist, never from input text.
	query := auctionSelect + " WHERE " + strings.Join(where, " AND ") +
		" ORDER BY " + Sorts[f.Sort] + ", a.id LIMIT " + fmt.Sprint(searchLimit)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Auction{}
	for rows.Next() {
		a, err := scanAuction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// alnum lowercases s and keeps only ASCII letters and digits, which also
// leaves nothing for LIKE to treat as a wildcard.
func alnum(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.ToLower(s))
}

// escapeLike makes user text match literally inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
