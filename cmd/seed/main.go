// Command seed fills the database with demo accounts and the data/seeds.json
// catalog as live auctions. Bids go through the bid service so wallet escrow
// and the ledger stay consistent. It is a no-op once the demo accounts exist,
// so it is safe to run on every container start.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ariefusry/diecast-auction/internal/db"
	"github.com/ariefusry/diecast-auction/internal/service"
	"github.com/ariefusry/diecast-auction/migrations"
)

// DemoPassword is shared by every demo account (demo_seller1, demo_bidder1, ...).
const DemoPassword = "demo-password"

const bidderFunds = 2 * service.MaxDeposit

var (
	basePrice  = map[string]int64{"Hot Wheels": 25_000, "Mini GT": 150_000, "Inno64": 300_000}
	conditions = []string{"mint_on_card", "sealed_box", "acrylic_case", "loose_mint", "damaged_packaging"}
	durations  = []int{30, 180, 720, 1440, 4320} // minutes; the short ones show settlement soon after seeding
)

func main() {
	file := flag.String("file", "data/seeds.json", "catalog to seed")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		log.Fatal(err)
	}
	var items []service.Item
	if err := json.Unmarshal(data, &items); err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(url, migrations.FS); err != nil {
		log.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	n, err := seed(context.Background(), pool, items, "demo_")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("seeded %d auctions", n)
}

// seed creates the demo users (prefix + seller1..2, bidder1..3) and one auction
// per item, returning how many it created. Returns 0 if the users already exist.
func seed(ctx context.Context, pool *pgxpool.Pool, items []service.Item, prefix string) (int, error) {
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE username = $1)", prefix+"seller1").
		Scan(&exists); err != nil || exists {
		return 0, err
	}

	auth, wallets, auctions := service.NewAuth(pool, nil), service.NewWallets(pool), service.NewAuctions(pool)
	register := func(name string) (int64, error) {
		u, err := auth.Register(ctx, prefix+name+"@demo.local", prefix+name, DemoPassword)
		return u.ID, err
	}
	var sellers, bidders []int64
	for _, name := range []string{"seller1", "seller2"} {
		id, err := register(name)
		if err != nil {
			return 0, err
		}
		sellers = append(sellers, id)
	}
	for _, name := range []string{"bidder1", "bidder2", "bidder3"} {
		id, err := register(name)
		if err != nil {
			return 0, err
		}
		for range bidderFunds / service.MaxDeposit {
			if _, err := wallets.Deposit(ctx, id, service.MaxDeposit); err != nil {
				return 0, err
			}
		}
		bidders = append(bidders, id)
	}

	// Every value derives from the item index, so a reseed of a fresh database is reproducible.
	for i, it := range items {
		base := basePrice[it.Brand]
		if base == 0 {
			base = 50_000
		}
		it.Condition = conditions[i%len(conditions)]
		in := service.NewAuction{
			Item:            it,
			StartPrice:      base + int64(i%5)*base/5,
			MinIncrement:    base / 5,
			DurationMinutes: durations[i%len(durations)],
		}
		if i%4 == 0 {
			in.BuyNowPrice = new(in.StartPrice * 3)
		}
		a, err := auctions.Create(ctx, sellers[i%len(sellers)], in)
		if err != nil {
			return i, err
		}
		if i%10 == 9 {
			continue // left as a draft
		}
		if _, err := auctions.Publish(ctx, a.SellerID, a.ID); err != nil {
			return i, err
		}
		if i%3 == 0 {
			continue // no bids yet, so buy-now stays available
		}
		for k := range 1 + i%3 {
			amount := in.StartPrice + int64(k)*in.MinIncrement
			if _, err := auctions.Bid(ctx, bidders[(i/3+k)%len(bidders)], a.ID, amount); err != nil {
				return i, err
			}
		}
	}
	return len(items), nil
}
