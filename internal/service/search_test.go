package service

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/testutil"
)

func TestNormalizeDropsValuesOutsideTheAllowedSets(t *testing.T) {
	got := Filter{
		Brand: "mini gt", Series: "  Car Culture ", Color: "Red", Tags: []string{"JDM", "", "jdm", "a", "b", "c", "d", "e"},
		Condition: "shiny", MinPrice: -5, MaxPrice: 100_000, Text: "  skyline   r34 ", Sort: "random",
	}.Normalize()
	want := Filter{
		Brand: "Mini GT", Series: "Car Culture", Color: "red", Tags: []string{"jdm", "a", "b", "c", "d"},
		MaxPrice: 100_000, Text: "skyline r34",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if f := (Filter{Brand: "Tomica", MinPrice: 200, MaxPrice: 100}).Normalize(); f.Brand != "" || f.MaxPrice != 0 {
		t.Fatalf("unknown brand / inverted range kept: %+v", f)
	}
	// Long or hostile input is cut cheaply and never carries NUL into SQL.
	if f := (Filter{Text: strings.Repeat("é", 200_000) + "\x00", Color: "re\x00d"}).Normalize(); len([]rune(f.Text)) != 100 || f.Color != "red" {
		t.Fatalf("long/NUL input = %d runes, color %q", len([]rune(f.Text)), f.Color)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`100%_off\`); got != `100\%\_off\\` {
		t.Fatalf("escapeLike = %q", got)
	}
}

// searchEnv seeds auctions whose model names share a unique token, so
// filters can be checked against only this test's rows.
func searchEnv(t *testing.T) (*auctionEnv, string, map[string]int64) {
	e := newAuctionEnv(t)
	seller, bidder := e.user(0), e.user(1_000_000)
	token := "Zq" + testutil.Unique()
	ids := map[string]int64{}
	for _, c := range []struct {
		key, brand, color, condition string
		tags                         []string
		start                        int64
	}{
		{"red-hw", "Hot Wheels", "Red", "mint_on_card", []string{"nissan", "jdm"}, 30_000},
		{"blue-mgt", "Mini GT", "Bayside Blue", "sealed_box", []string{"nissan", "jdm"}, 150_000},
		{"red-mgt", "Mini GT", "Candy Red", "loose_mint", []string{"ferrari"}, 200_000},
	} {
		in := sampleAuction()
		in.Item = Item{Brand: c.brand, ModelName: token + " " + c.key, Color: c.color, Condition: c.condition, Tags: c.tags}
		in.StartPrice = c.start
		ids[c.key] = e.active(seller, in).ID
	}
	// A bid lifts red-mgt's current price above its start price.
	if _, err := e.auctions.Bid(context.Background(), bidder, ids["red-mgt"], 260_000); err != nil {
		t.Fatalf("Bid: %v", err)
	}
	e.draft(seller, NewAuction{Item: Item{Brand: "Hot Wheels", ModelName: token + " draft", Condition: "loose_mint"},
		StartPrice: 1_000, MinIncrement: 1_000, DurationMinutes: 60})
	return e, token, ids
}

func TestSearchAppliesFilters(t *testing.T) {
	e, token, ids := searchEnv(t)
	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"text only, drafts hidden", Filter{Text: token, Sort: "price_asc"}, []string{"red-hw", "blue-mgt", "red-mgt"}},
		{"brand", Filter{Text: token, Brand: "Mini GT", Sort: "price_asc"}, []string{"blue-mgt", "red-mgt"}},
		{"color substring", Filter{Text: token, Color: "red", Sort: "price_desc"}, []string{"red-mgt", "red-hw"}},
		{"tags", Filter{Text: token, Tags: []string{"jdm"}, Sort: "price_asc"}, []string{"red-hw", "blue-mgt"}},
		{"condition", Filter{Text: token, Condition: "sealed_box"}, []string{"blue-mgt"}},
		{"price uses current bid", Filter{Text: token, MinPrice: 250_000}, []string{"red-mgt"}},
		{"max price", Filter{Text: token, MaxPrice: 160_000, Sort: "price_asc"}, []string{"red-hw", "blue-mgt"}},
		{"words match in any order", Filter{Text: "blue-mgt " + token}, []string{"blue-mgt"}},
		{"punctuation ignored", Filter{Text: token + " bluemgt"}, []string{"blue-mgt"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := e.auctions.Search(context.Background(), c.f, true)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if res.Relaxed {
				t.Fatalf("strict match reported as relaxed")
			}
			assertIDs(t, res.Auctions, ids, c.want)
		})
	}
}

func TestSearchRelaxesWhenNothingMatches(t *testing.T) {
	e, token, ids := searchEnv(t)
	ctx := context.Background()

	// No purple car: color is dropped, brand and price still apply.
	// The catalog's explicit filters never relax.
	if res, _ := e.auctions.Search(ctx, Filter{Text: token, Color: "purple"}, false); res.Relaxed || len(res.Auctions) != 0 {
		t.Fatalf("strict search relaxed: %+v", res)
	}

	res, err := e.auctions.Search(ctx, Filter{Text: token, Brand: "Mini GT", Color: "purple", MaxPrice: 160_000}, true)
	if err != nil || !res.Relaxed {
		t.Fatalf("Search = %+v, %v; want relaxed", res, err)
	}
	assertIDs(t, res.Auctions, ids, []string{"blue-mgt"})

	// Still nothing within budget: price is dropped too, brand stays.
	res, _ = e.auctions.Search(ctx, Filter{Text: token, Brand: "Mini GT", Color: "purple", MaxPrice: 10_000, Sort: "price_asc"}, true)
	if !res.Relaxed {
		t.Fatal("want relaxed")
	}
	assertIDs(t, res.Auctions, ids, []string{"blue-mgt", "red-mgt"})
}

func assertIDs(t *testing.T, got []Auction, ids map[string]int64, want []string) {
	t.Helper()
	var gotIDs, wantIDs []int64
	for _, a := range got {
		gotIDs = append(gotIDs, a.ID)
	}
	for _, k := range want {
		wantIDs = append(wantIDs, ids[k])
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("got ids %v, want %v (%v)", gotIDs, wantIDs, want)
	}
}

// A raw sentence (the no-LLM fallback) still finds items through any one word.
func TestSearchFallsBackToAnyWord(t *testing.T) {
	e, token, ids := searchEnv(t)
	res, err := e.auctions.Search(context.Background(), Filter{Text: "carikan " + token + " di bawah", Sort: "price_asc"}, true)
	if err != nil || !res.Relaxed {
		t.Fatalf("Search = %+v, %v; want relaxed", res, err)
	}
	assertIDs(t, res.Auctions, ids, []string{"red-hw", "blue-mgt", "red-mgt"})
}
