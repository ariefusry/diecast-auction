// Command scraper pulls real 1:64 castings from the Hot Wheels, Mini GT,
// Inno64, Matchbox, Tomica, Tarmac Works and M2 Machines Fandom wikis into data/seeds.json. It uses the MediaWiki API
// (the HTML pages sit behind a Cloudflare challenge), one request per second.
// Wiki content is CC-BY-SA, so every item keeps its page URL as source_url.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ariefusry/diecast-auction/internal/service"
)

const userAgent = "diecast-auction-scraper/0.1 (portfolio project; https://github.com/ariefusry/diecast-auction)"

type source struct {
	wiki, brand string
	category    string // empty means every article page
	limit       int
}

var sources = []source{
	{"hotwheels", "Hot Wheels", "JDM Cars", 40},
	{"minigt", "Mini GT", "Models", 40},
	{"inno64", "Inno64", "", 40},
	{"matchbox", "Matchbox", "1:64", 40},
	{"tomica", "Tomica", "Scale 1/64", 40},
	{"tarmacworks", "Tarmac Works", "", 40},
	{"m2machines", "M2 Machines", "164th", 40},
}

func main() {
	out := flag.String("out", "data/seeds.json", "output file")
	flag.Parse()

	var items []service.Item
	for _, s := range sources {
		got, err := scrape(s)
		if err != nil {
			log.Fatalf("%s: %v", s.wiki, err)
		}
		log.Printf("%s: %d items", s.wiki, len(got))
		items = append(items, got...)
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d items to %s", len(items), *out)
}

func scrape(s source) ([]service.Item, error) {
	titles, err := listTitles(s)
	if err != nil {
		return nil, err
	}
	// Fetch more than needed: pages without a photo are dropped.
	titles = pick(titles, s.limit*2)
	var items []service.Item
	for batch := range slices.Chunk(titles, 50) {
		var resp struct {
			Query struct {
				Pages []struct {
					Title     string
					FullURL   string
					Missing   bool
					Original  struct{ Source string }
					Revisions []struct {
						Slots struct{ Main struct{ Content string } }
					}
				}
			}
		}
		err := api(s.wiki, url.Values{
			"action": {"query"}, "titles": {strings.Join(batch, "|")},
			"prop": {"revisions|pageimages|info"}, "rvprop": {"content"}, "rvslots": {"main"},
			"piprop": {"original"}, "inprop": {"url"},
		}, &resp)
		if err != nil {
			return nil, err
		}
		for _, p := range resp.Query.Pages {
			if p.Missing || p.Original.Source == "" || len(p.Revisions) == 0 {
				continue
			}
			it := parsePage(s.brand, p.Title, p.Revisions[0].Slots.Main.Content)
			// Tarmac Works wheel sets (PARTS64) and its 1:43 line (HOBBY43).
			if it.Series == "PARTS64" || strings.Contains(it.Series, "43") {
				continue
			}
			it.ImageURL, it.SourceURL = p.Original.Source, p.FullURL
			items = append(items, it)
		}
	}
	return pick(items, s.limit), nil
}

func listTitles(s source) ([]string, error) {
	params := url.Values{"action": {"query"}}
	if s.category == "" {
		params.Set("list", "allpages")
		params.Set("aplimit", "500")
	} else {
		params.Set("list", "categorymembers")
		params.Set("cmtitle", "Category:"+s.category)
		params.Set("cmnamespace", "0")
		params.Set("cmlimit", "500")
	}
	// MediaWiki returns at most 500 titles per call; follow "continue" until done,
	// otherwise an alphabetical list stops partway through the letters.
	var titles []string
	for {
		var resp struct {
			Continue map[string]string
			Query    struct {
				AllPages        []struct{ Title string }
				CategoryMembers []struct{ Title string }
			}
		}
		if err := api(s.wiki, params, &resp); err != nil {
			return nil, err
		}
		for _, p := range append(resp.Query.AllPages, resp.Query.CategoryMembers...) {
			if p.Title != "Main Page" && !strings.HasSuffix(p.Title, " Wiki") && isCasting(p.Title) {
				titles = append(titles, p.Title)
			}
		}
		if len(resp.Continue) == 0 {
			return titles, nil
		}
		for k, v := range resp.Continue {
			params.Set(k, v)
		}
	}
}

// pick returns up to n elements spread evenly, so an alphabetical list does
// not yield only the A-L half or forty variants of the same make.
func pick[T any](xs []T, n int) []T {
	if len(xs) <= n {
		return xs
	}
	out := make([]T, n)
	for i := range out {
		out[i] = xs[i*len(xs)/n]
	}
	return out
}

// isCasting drops pages that are not single 1:64 castings: other scales such
// as "(1:20)" or Tarmac Works' T43 line, cancelled releases, and a make's own
// index page such as "Mazda".
func isCasting(title string) bool {
	if strings.Contains(title, "(1:") || strings.HasPrefix(title, "T43") || strings.Contains(title, "Cancelled") {
		return false
	}
	w := words(title)
	return !(len(w) == 1 && makeIn(title) != "")
}

var lastRequest time.Time

// api calls the wiki's api.php with a polite delay and decodes the JSON into dst.
func api(wiki string, params url.Values, dst any) error {
	time.Sleep(time.Until(lastRequest.Add(time.Second)))
	lastRequest = time.Now()
	params.Set("format", "json")
	params.Set("formatversion", "2")
	req, err := http.NewRequest("GET", "https://"+wiki+".fandom.com/api.php?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api.php: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

var (
	infoboxYear   = regexp.MustCompile(`(?i)\|\s*years?\s*=\s*\[*(\d{4})`)
	infoboxSeries = regexp.MustCompile(`(?i)\|\s*series\s*=\s*((?:\[\[[^\]]*\]\]|[^\n|}])+)`)
	fourDigits    = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	// Tomica pages only say "was released July 16, 2016" in prose.
	releasedYear  = regexp.MustCompile(`released [^.]*?\b((?:19|20)\d{2})\b`)
	japaneseMakes = []string{"acura", "datsun", "daihatsu", "honda", "infiniti", "isuzu", "lexus",
		"mazda", "mitsubishi", "nissan", "subaru", "suzuki", "toyota"}
	// makes maps a title word to its make; aliases cover model-only names like "Skyline".
	makes = map[string]string{
		"skyline": "nissan", "prince": "nissan", "silvia": "nissan", "eunos": "mazda", "lancer": "mitsubishi",
		"altezza": "toyota", "supra": "toyota", "chevy": "chevrolet", "vw": "volkswagen",
		"mercedes": "mercedes-benz", "mercedes-amg": "mercedes-benz", "land": "land rover", "aston": "aston martin", "alfa": "alfa romeo",
	}
	colors = []string{"white", "black", "red", "blue", "green", "yellow", "orange", "silver",
		"grey", "gray", "gold", "purple", "pink", "brown", "bronze"}
)

func init() {
	for _, m := range append(japaneseMakes, "abarth", "audi", "bentley", "bmw", "bugatti", "buick",
		"cadillac", "chevrolet", "dodge", "ferrari", "fiat", "ford", "jaguar", "jeep", "lamborghini",
		"lancia", "lincoln", "lotus", "maserati", "mclaren", "mercedes-benz", "pagani", "peugeot",
		"plymouth", "pontiac", "porsche", "renault", "shelby", "tesla", "volkswagen", "volvo") {
		makes[m] = m
	}
}

// words splits a title into lowercase words, keeping hyphens (mercedes-benz).
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// makeIn returns the first known make in the title, skipping tuner prefixes like "HKS".
func makeIn(title string) string {
	for _, w := range words(title) {
		if m, ok := makes[w]; ok {
			return m
		}
	}
	return ""
}

// colorIn returns the first basic color word in the title, as written there.
func colorIn(title string) string {
	for _, w := range words(title) {
		if slices.Contains(colors, w) {
			return strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return ""
}

// parsePage reads one casting page. Year and series prefer the infobox; the
// first row of the versions table fills in color and anything missing.
func parsePage(brand, title, text string) service.Item {
	it := service.Item{Brand: brand, ModelName: strings.TrimPrefix(title, "1/64 ")}
	if m := infoboxYear.FindStringSubmatch(text); m != nil {
		it.Year, _ = strconv.Atoi(m[1])
	}
	if m := infoboxSeries.FindStringSubmatch(text); m != nil {
		it.Series = clean(m[1])
	}
	row := firstRow(text, "Color", "Livery")
	if len(row) == 0 {
		// Tarmac Works titles pages by SKU and writes "[[Model]] <br/> Color"
		// in one Description cell.
		row = firstRow(brTag.ReplaceAllString(text, " ¦ "), "Description")
		// On SKU pages the last line is the color and earlier lines name the
		// model; a page titled by model lists only colors there.
		d := row["description"]
		if sku.MatchString(title) {
			i := strings.LastIndex(d, "¦")
			if i < 0 {
				i = len(d) // a single line is just the model
			}
			it.ModelName = strings.Join(strings.Fields(strings.ReplaceAll(d[:i], "¦", "")), " ")
			d = strings.TrimPrefix(d[i:], "¦")
		}
		row["color"] = strings.Join(strings.Fields(strings.ReplaceAll(d, "¦", "")), " ")
		// Some series names are spelled one letter per line (G<br/>L<br/>O...).
		row["series"] = strings.ReplaceAll(row["series"], " ¦ ", "")
	}
	if it.Year == 0 {
		for _, k := range []string{"year", "prod. yr", "date"} {
			if y := fourDigits.FindString(row[k]); y != "" {
				it.Year, _ = strconv.Atoi(y)
				break
			}
		}
	}
	if it.Year == 0 {
		if m := releasedYear.FindStringSubmatch(text); m != nil {
			it.Year, _ = strconv.Atoi(m[1])
		}
	}
	if it.Series == "" {
		it.Series = row["series"]
	}
	it.Color = row["color"]
	if it.Color == "" {
		it.Color = row["livery"]
	}

	if it.Color == "" {
		it.Color = colorIn(title)
	}
	it.Tags = []string{}
	if m := makeIn(it.ModelName); m != "" {
		it.Tags = append(it.Tags, m)
		if slices.Contains(japaneseMakes, m) {
			it.Tags = append(it.Tags, "jdm")
		}
	}
	return it
}

// firstRow returns the first data row of the first wiki table that has one of
// the given columns, keyed by lowercased header text.
func firstRow(text string, columns ...string) map[string]string {
	for _, table := range tables(text) {
		rows := strings.Split(table, "\n|-")
		// The header is the first row with cells; some tables open with a bare |-.
		h := slices.IndexFunc(rows, func(r string) bool { return len(cells(r)) > 0 })
		if h < 0 {
			continue
		}
		header := cells(rows[h])
		if !slices.ContainsFunc(columns, func(c string) bool { return slices.Contains(header, c) }) {
			continue
		}
		for _, r := range rows[h+1:] {
			if c := cells(r); len(c) == len(header) && !strings.HasPrefix(strings.TrimSpace(r), "!") {
				row := map[string]string{}
				for i, h := range header {
					row[strings.ToLower(h)] = c[i]
				}
				return row
			}
		}
	}
	return map[string]string{}
}

func tables(text string) []string {
	var out []string
	for {
		start := strings.Index(text, "{|")
		if start < 0 {
			return out
		}
		end := strings.Index(text[start:], "\n|}")
		if end < 0 {
			return out
		}
		out = append(out, text[start:start+end])
		text = text[start+end+3:]
	}
}

// cells splits one table row into cleaned cell values. Lines that start with
// neither "|" nor "!" continue the previous cell.
func cells(row string) []string {
	var out []string
	for _, line := range strings.Split(row, "\n") {
		switch {
		case strings.HasPrefix(line, "{|"), strings.HasPrefix(line, "|+"), strings.HasPrefix(line, "|-"):
		case strings.HasPrefix(line, "|"), strings.HasPrefix(line, "!"):
			for _, c := range cellSep.Split(line[1:], -1) {
				// Drop the attribute part of `style="..." |value`.
				if i := strings.Index(c, "|"); i >= 0 && strings.Contains(c[:i], "=") && !strings.Contains(c[:i], "[[") {
					c = c[i+1:]
				}
				out = append(out, clean(c))
			}
		case len(out) > 0:
			out[len(out)-1] = strings.TrimSpace(out[len(out)-1] + " " + clean(line))
		}
	}
	return out
}

var (
	cellSep  = regexp.MustCompile(`\|\||!!`)
	brTag    = regexp.MustCompile(`<br\s*/?>`)
	sku      = regexp.MustCompile(`^(T\d+[A-Z]*-|MGT\d)`)
	fileLink = regexp.MustCompile(`\[\[(?:File|Image):[^\]]*\]\]`)
	wikiLink = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`)
	template = regexp.MustCompile(`\{\{[^}]*\}\}`)
	htmlTag  = regexp.MustCompile(`<[^>]+>`)
)

// clean turns wikitext into plain text.
func clean(s string) string {
	s = fileLink.ReplaceAllString(s, "")
	s = wikiLink.ReplaceAllString(s, "$1")
	s = template.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "<br>", " ")
	s = htmlTag.ReplaceAllString(s, "")
	s = strings.NewReplacer("'''", "", "''", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
