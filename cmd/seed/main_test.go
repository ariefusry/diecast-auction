package main

import (
	"context"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/service"
	"github.com/ariefusry/diecast-auction/internal/testutil"
)

func TestSeedBuildsConsistentDemoData(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	prefix := "t" + testutil.Unique() + "_"
	var items []service.Item
	for i, brand := range []string{"Hot Wheels", "Mini GT", "Inno64", "Hot Wheels", "Mini GT",
		"Inno64", "Hot Wheels", "Mini GT", "Inno64", "Hot Wheels"} {
		items = append(items, service.Item{Brand: brand, ModelName: "Model " + string(rune('A'+i)), Tags: []string{"jdm"}})
	}

	n, err := seed(ctx, pool, items, prefix)
	if err != nil || n != len(items) {
		t.Fatalf("seed = %d, %v; want %d", n, err, len(items))
	}

	var users []int64
	rows, _ := pool.Query(ctx, "SELECT id FROM users WHERE username LIKE $1 || '%'", prefix)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		users = append(users, id)
	}
	if len(users) != 5 {
		t.Fatalf("demo users = %d, want 5", len(users))
	}
	for _, id := range users {
		testutil.AssertLedger(t, pool, id)
	}

	var auctions, drafts, withBids, bids int
	err = pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE a.status = 'draft'),
		       count(*) FILTER (WHERE a.highest_bidder_id IS NOT NULL),
		       (SELECT count(*) FROM bids b JOIN auctions a2 ON a2.id = b.auction_id
		        JOIN items i2 ON i2.id = a2.item_id WHERE i2.seller_id = ANY($1))
		FROM auctions a JOIN items i ON i.id = a.item_id WHERE i.seller_id = ANY($1)`, users).
		Scan(&auctions, &drafts, &withBids, &bids)
	if err != nil || auctions != len(items) || drafts != 1 || withBids == 0 || bids <= withBids {
		t.Fatalf("auctions=%d drafts=%d withBids=%d bids=%d err=%v", auctions, drafts, withBids, bids, err)
	}

	// Buy-now is only possible before the first bid, so a seeded lot with both is dead weight.
	var dead int
	pool.QueryRow(ctx, `SELECT count(*) FROM auctions a JOIN items i ON i.id = a.item_id
		WHERE i.seller_id = ANY($1) AND a.buy_now_price IS NOT NULL AND a.highest_bidder_id IS NOT NULL`, users).Scan(&dead)
	if dead != 0 {
		t.Fatalf("%d seeded auctions have buy-now and bids", dead)
	}

	// Running again (every container start) must not duplicate anything.
	if n, err := seed(ctx, pool, items, prefix); err != nil || n != 0 {
		t.Fatalf("second seed = %d, %v; want 0", n, err)
	}
}
