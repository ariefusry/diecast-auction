package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ariefusry/diecast-auction/internal/testutil"
)

type auctionEnv struct {
	t        *testing.T
	auctions *Auctions
	wallets  *Wallets
	auth     *Auth
}

func newAuctionEnv(t *testing.T) *auctionEnv {
	pool := testutil.Pool(t)
	return &auctionEnv{t: t, auctions: NewAuctions(pool), wallets: NewWallets(pool), auth: NewAuth(pool, testSecret)}
}

// user registers a user and deposits balance (0 means no deposit).
func (e *auctionEnv) user(balance int64) int64 {
	e.t.Helper()
	u, _ := registerUser(e.t, e.auth)
	for balance > 0 {
		amount := min(balance, MaxDeposit)
		if _, err := e.wallets.Deposit(context.Background(), u.ID, amount); err != nil {
			e.t.Fatalf("Deposit: %v", err)
		}
		balance -= amount
	}
	return u.ID
}

func sampleAuction() NewAuction {
	return NewAuction{
		Item: Item{
			Brand: "Mini GT", ModelName: "Nissan Skyline GT-R R34", Series: "MGT", Year: 2023,
			Color: "Bayside Blue", Tags: []string{"jdm"}, Condition: "sealed_box",
			SourceURL: "https://minigt.fandom.com/wiki/Nissan_Skyline",
		},
		StartPrice: 100_000, MinIncrement: 10_000, DurationMinutes: 60,
	}
}

func (e *auctionEnv) draft(sellerID int64, in NewAuction) Auction {
	e.t.Helper()
	a, err := e.auctions.Create(context.Background(), sellerID, in)
	if err != nil {
		e.t.Fatalf("Create: %v", err)
	}
	return a
}

func (e *auctionEnv) active(sellerID int64, in NewAuction) Auction {
	e.t.Helper()
	a, err := e.auctions.Publish(context.Background(), sellerID, e.draft(sellerID, in).ID)
	if err != nil {
		e.t.Fatalf("Publish: %v", err)
	}
	return a
}

func (e *auctionEnv) wallet(userID int64) Wallet {
	e.t.Helper()
	w, err := e.wallets.Get(context.Background(), userID)
	if err != nil {
		e.t.Fatalf("Get wallet: %v", err)
	}
	return w
}

func (e *auctionEnv) assertLedger(userIDs ...int64) {
	e.t.Helper()
	for _, id := range userIDs {
		testutil.AssertLedger(e.t, e.auctions.pool, id)
	}
}

// setEndTime moves end_time relative to the database clock.
func (e *auctionEnv) setEndTime(auctionID int64, fromNow string) {
	e.t.Helper()
	_, err := e.auctions.pool.Exec(context.Background(),
		"UPDATE auctions SET end_time = now() + $2::interval WHERE id = $1", auctionID, fromNow)
	if err != nil {
		e.t.Fatalf("set end_time: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestCreateAuctionStartsAsDraft(t *testing.T) {
	e := newAuctionEnv(t)
	seller := e.user(0)

	in := sampleAuction()
	in.Item.Tags = []string{"JDM"}
	in.Item.ImageURL = "/uploads/abc.png"
	a := e.draft(seller, in)
	if a.ID == 0 || a.SellerID != seller || a.Status != "draft" || a.StartTime != nil || a.EndTime != nil {
		t.Fatalf("draft = %+v", a)
	}
	if a.DurationMinutes != 60 || a.Item.ModelName != "Nissan Skyline GT-R R34" || a.Item.Tags[0] != "jdm" || a.Item.ImageURL != "/uploads/abc.png" ||
		a.Item.SourceURL != "https://minigt.fandom.com/wiki/Nissan_Skyline" {
		t.Fatalf("draft fields = %+v", a)
	}
	got, err := e.auctions.Get(context.Background(), a.ID)
	if err != nil || got.ID != a.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := e.auctions.Get(context.Background(), -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get unknown err = %v", err)
	}
}

func TestCreateAuctionValidates(t *testing.T) {
	e := newAuctionEnv(t)
	seller := e.user(0)
	cases := map[string]func(*NewAuction){
		"no brand":            func(n *NewAuction) { n.Item.Brand = " " },
		"no model":            func(n *NewAuction) { n.Item.ModelName = "" },
		"bad condition":       func(n *NewAuction) { n.Item.Condition = "shiny" },
		"zero start":          func(n *NewAuction) { n.StartPrice = 0 },
		"zero increment":      func(n *NewAuction) { n.MinIncrement = 0 },
		"buy now below start": func(n *NewAuction) { n.BuyNowPrice = ptr(int64(100_000)) },
		"zero duration":       func(n *NewAuction) { n.DurationMinutes = 0 },
		"too long":            func(n *NewAuction) { n.DurationMinutes = MaxDurationMinutes + 1 },
		"unknown brand":       func(n *NewAuction) { n.Item.Brand = "Tomica" },
		"year out of range":   func(n *NewAuction) { n.Item.Year = 3_000_000_000 },
		"NUL byte":            func(n *NewAuction) { n.Item.ModelName = "R34\x00" },
		"model too long":      func(n *NewAuction) { n.Item.ModelName = strings.Repeat("x", 201) },
		"too many tags":       func(n *NewAuction) { n.Item.Tags = strings.Fields("a b c d e f g h i j k") },
		"script source url":   func(n *NewAuction) { n.Item.SourceURL = "javascript:alert(1)" },
		"script image url":    func(n *NewAuction) { n.Item.ImageURL = "javascript:alert(1)" },
		"relative image path": func(n *NewAuction) { n.Item.ImageURL = "/etc/passwd" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := sampleAuction()
			mutate(&in)
			if _, err := e.auctions.Create(context.Background(), seller, in); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestPublishOpensBiddingWindow(t *testing.T) {
	e := newAuctionEnv(t)
	seller, other := e.user(0), e.user(0)
	d := e.draft(seller, sampleAuction())
	ctx := context.Background()

	if _, err := e.auctions.Publish(ctx, other, d.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("publish by non-seller err = %v", err)
	}
	a, err := e.auctions.Publish(ctx, seller, d.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if a.Status != "active" || a.StartTime == nil || a.EndTime == nil || a.EndTime.Sub(*a.StartTime) != time.Hour {
		t.Fatalf("published = %+v", a)
	}
	if _, err := e.auctions.Publish(ctx, seller, d.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("second publish err = %v", err)
	}
	if _, err := e.auctions.Publish(ctx, seller, -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("publish unknown err = %v", err)
	}
}

func TestCancelOnlyBeforeFirstBid(t *testing.T) {
	e := newAuctionEnv(t)
	seller, bidder := e.user(0), e.user(1_000_000)
	ctx := context.Background()

	draft := e.draft(seller, sampleAuction())
	if _, err := e.auctions.Cancel(ctx, bidder, draft.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cancel by non-seller err = %v", err)
	}
	if a, err := e.auctions.Cancel(ctx, seller, draft.ID); err != nil || a.Status != "cancelled" {
		t.Fatalf("cancel draft = %+v, %v", a, err)
	}
	if _, err := e.auctions.Cancel(ctx, seller, draft.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("cancel twice err = %v", err)
	}

	active := e.active(seller, sampleAuction())
	if _, err := e.auctions.Bid(ctx, bidder, active.ID, 100_000); err != nil {
		t.Fatalf("Bid: %v", err)
	}
	if _, err := e.auctions.Cancel(ctx, seller, active.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("cancel with bid err = %v", err)
	}
}

func TestBidEscrowsAndReleasesOutbidFunds(t *testing.T) {
	e := newAuctionEnv(t)
	seller, alice, bob := e.user(0), e.user(500_000), e.user(500_000)
	a := e.active(seller, sampleAuction())
	ctx := context.Background()

	got, err := e.auctions.Bid(ctx, alice, a.ID, 100_000)
	if err != nil {
		t.Fatalf("alice bid: %v", err)
	}
	if *got.CurrentHighestBid != 100_000 || *got.HighestBidderID != alice {
		t.Fatalf("after alice = %+v", got)
	}
	if w := e.wallet(alice); w != (Wallet{Available: 400_000, Locked: 100_000}) {
		t.Fatalf("alice wallet = %+v", w)
	}

	if _, err := e.auctions.Bid(ctx, bob, a.ID, 110_000); err != nil {
		t.Fatalf("bob bid: %v", err)
	}
	if w := e.wallet(alice); w != (Wallet{Available: 500_000}) {
		t.Fatalf("alice after outbid = %+v", w)
	}
	if w := e.wallet(bob); w != (Wallet{Available: 390_000, Locked: 110_000}) {
		t.Fatalf("bob wallet = %+v", w)
	}
	e.assertLedger(alice, bob, seller)

	var bids int
	e.auctions.pool.QueryRow(ctx, "SELECT count(*) FROM bids WHERE auction_id = $1", a.ID).Scan(&bids)
	if bids != 2 {
		t.Fatalf("bids rows = %d, want 2", bids)
	}
}

func TestBidValidation(t *testing.T) {
	e := newAuctionEnv(t)
	seller, alice, bob, poor := e.user(0), e.user(1_000_000), e.user(1_000_000), e.user(50_000)
	ctx := context.Background()

	open := e.active(seller, sampleAuction())
	if _, err := e.auctions.Bid(ctx, alice, open.ID, 100_000); err != nil {
		t.Fatalf("Bid: %v", err)
	}
	draft := e.draft(seller, sampleAuction())
	ended := e.active(seller, sampleAuction())
	e.setEndTime(ended.ID, "-1 second")
	fresh := e.active(seller, sampleAuction())

	cases := []struct {
		name      string
		bidder    int64
		auctionID int64
		amount    int64
		want      error
	}{
		{"unknown auction", bob, -1, 200_000, ErrNotFound},
		{"draft", bob, draft.ID, 200_000, ErrConflict},
		{"ended", bob, ended.ID, 200_000, ErrConflict},
		{"seller", seller, open.ID, 200_000, ErrForbidden},
		{"non-positive", bob, open.ID, 0, ErrInvalidInput},
		{"first below start", bob, fresh.ID, 99_999, ErrConflict},
		{"below increment", bob, open.ID, 109_999, ErrConflict},
		{"already highest", alice, open.ID, 200_000, ErrConflict},
		{"insufficient", poor, fresh.ID, 100_000, ErrConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := e.auctions.Bid(ctx, c.bidder, c.auctionID, c.amount); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
	e.assertLedger(alice, bob, poor)
}

func TestBidInFinalTwoMinutesExtendsEndTime(t *testing.T) {
	e := newAuctionEnv(t)
	seller, alice := e.user(0), e.user(1_000_000)
	ctx := context.Background()

	closing := e.active(seller, sampleAuction())
	e.setEndTime(closing.ID, "1 minute")
	before, _ := e.auctions.Get(ctx, closing.ID)
	after, err := e.auctions.Bid(ctx, alice, closing.ID, 100_000)
	if err != nil {
		t.Fatalf("Bid: %v", err)
	}
	if d := after.EndTime.Sub(*before.EndTime); d != 2*time.Minute {
		t.Fatalf("closing end_time moved %v, want 2m", d)
	}

	calm := e.active(seller, sampleAuction())
	after, err = e.auctions.Bid(ctx, alice, calm.ID, 100_000)
	if err != nil {
		t.Fatalf("Bid: %v", err)
	}
	if !after.EndTime.Equal(*calm.EndTime) {
		t.Fatalf("calm end_time moved from %v to %v", calm.EndTime, after.EndTime)
	}
}

func TestBuyNow(t *testing.T) {
	e := newAuctionEnv(t)
	seller, buyer, poor := e.user(0), e.user(1_000_000), e.user(100_000)
	ctx := context.Background()
	withBuyNow := sampleAuction()
	withBuyNow.BuyNowPrice = ptr(int64(300_000))

	a := e.active(seller, withBuyNow)
	if _, err := e.auctions.BuyNow(ctx, seller, a.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("seller buy-now err = %v", err)
	}
	if _, err := e.auctions.BuyNow(ctx, poor, a.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("poor buy-now err = %v", err)
	}
	got, err := e.auctions.BuyNow(ctx, buyer, a.ID)
	if err != nil {
		t.Fatalf("BuyNow: %v", err)
	}
	if got.Status != "completed" || got.WinnerID == nil || *got.WinnerID != buyer {
		t.Fatalf("after buy-now = %+v", got)
	}
	if w := e.wallet(buyer); w != (Wallet{Available: 700_000}) {
		t.Fatalf("buyer wallet = %+v", w)
	}
	if w := e.wallet(seller); w != (Wallet{Available: 300_000}) {
		t.Fatalf("seller wallet = %+v", w)
	}
	if _, err := e.auctions.BuyNow(ctx, buyer, a.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("second buy-now err = %v", err)
	}
	e.assertLedger(seller, buyer, poor)

	noPrice := e.active(seller, sampleAuction())
	if _, err := e.auctions.BuyNow(ctx, buyer, noPrice.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("buy-now without price err = %v", err)
	}
	bidded := e.active(seller, withBuyNow)
	if _, err := e.auctions.Bid(ctx, poor, bidded.ID, 100_000); err != nil {
		t.Fatalf("Bid: %v", err)
	}
	if _, err := e.auctions.BuyNow(ctx, buyer, bidded.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("buy-now after bid err = %v", err)
	}
}

// race runs each fn concurrently. Domain sentinels are expected losses; any
// other error (for example a deadlock, SQLSTATE 40P01) fails the test.
func race(t *testing.T, fns ...func() error) (wins int) {
	t.Helper()
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Go(func() {
			err := fn()
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case !errors.Is(err, ErrConflict):
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()
	return wins
}

// Many bidders hammer two auctions at once. Each transaction locks the
// auction and then wallets by ascending user_id, so there must be no
// deadlock, no lost escrow, and only each auction's leader holds locked funds.
func TestConcurrentBidsKeepEscrowConsistent(t *testing.T) {
	e := newAuctionEnv(t)
	const funded = 50_000_000
	seller := e.user(0)
	bidders := make([]int64, 8)
	for i := range bidders {
		bidders[i] = e.user(funded)
	}
	auctions := []Auction{e.active(seller, sampleAuction()), e.active(seller, sampleAuction())}
	ctx := context.Background()

	var fns []func() error
	for round := range 5 {
		for i, bidder := range bidders {
			for k, a := range auctions {
				amount := int64(100_000 + (round*len(bidders)+i)*20_000 + k*10_000)
				fns = append(fns, func() error {
					_, err := e.auctions.Bid(ctx, bidder, a.ID, amount)
					return err
				})
			}
		}
	}
	if wins := race(t, fns...); wins == 0 {
		t.Fatal("no bid succeeded")
	}

	leaders := map[int64]int64{} // bidder -> total locked they should hold
	for _, a := range auctions {
		got, _ := e.auctions.Get(ctx, a.ID)
		var maxBid int64
		e.auctions.pool.QueryRow(ctx, "SELECT max(amount) FROM bids WHERE auction_id = $1", a.ID).Scan(&maxBid)
		if *got.CurrentHighestBid != maxBid {
			t.Errorf("auction %d highest = %d, max bid row = %d", a.ID, *got.CurrentHighestBid, maxBid)
		}
		leaders[*got.HighestBidderID] += *got.CurrentHighestBid
	}
	for _, b := range bidders {
		w := e.wallet(b)
		if w.Available+w.Locked != funded || w.Locked != leaders[b] {
			t.Errorf("bidder %d wallet = %+v, want locked %d and total %d", b, w, leaders[b], funded)
		}
	}
	e.assertLedger(bidders...)
}

func TestConcurrentBuyNowAndFirstBid(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	in := sampleAuction()
	in.BuyNowPrice = ptr(int64(300_000))

	for range 10 {
		seller, buyer, bidder := e.user(0), e.user(1_000_000), e.user(1_000_000)
		a := e.active(seller, in)
		race(t,
			func() error { _, err := e.auctions.BuyNow(ctx, buyer, a.ID); return err },
			func() error { _, err := e.auctions.Bid(ctx, bidder, a.ID, 100_000); return err },
		)
		got, _ := e.auctions.Get(ctx, a.ID)
		sold := got.Status == "completed" && got.HighestBidderID == nil
		bid := got.Status == "active" && got.HighestBidderID != nil && *got.HighestBidderID == bidder
		if sold == bid {
			t.Fatalf("auction %d in mixed state %+v", a.ID, got)
		}
		total := e.wallet(seller).Available + e.wallet(buyer).Available + e.wallet(bidder).Available + e.wallet(bidder).Locked
		if total != 2_000_000 {
			t.Fatalf("money not conserved: %d", total)
		}
		e.assertLedger(seller, buyer, bidder)
	}
}

func TestConcurrentBuyNowSellsOnce(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	in := sampleAuction()
	in.BuyNowPrice = ptr(int64(300_000))
	seller := e.user(0)
	a := e.active(seller, in)

	var fns []func() error
	buyers := make([]int64, 5)
	for i := range buyers {
		buyers[i] = e.user(1_000_000)
		fns = append(fns, func() error { _, err := e.auctions.BuyNow(ctx, buyers[i], a.ID); return err })
	}
	if wins := race(t, fns...); wins != 1 {
		t.Fatalf("buy-now succeeded %d times, want 1", wins)
	}
	if w := e.wallet(seller); w.Available != 300_000 {
		t.Fatalf("seller wallet = %+v", w)
	}
	e.assertLedger(append(buyers, seller)...)
}

func TestConcurrentCancelAndFirstBid(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()

	for range 10 {
		seller, bidder := e.user(0), e.user(1_000_000)
		a := e.active(seller, sampleAuction())
		race(t,
			func() error { _, err := e.auctions.Cancel(ctx, seller, a.ID); return err },
			func() error { _, err := e.auctions.Bid(ctx, bidder, a.ID, 100_000); return err },
		)
		got, _ := e.auctions.Get(ctx, a.ID)
		w := e.wallet(bidder)
		cancelled := got.Status == "cancelled" && got.HighestBidderID == nil && w.Locked == 0
		bid := got.Status == "active" && got.HighestBidderID != nil && w.Locked == 100_000
		if cancelled == bid {
			t.Fatalf("auction %d in mixed state %+v, bidder wallet %+v", a.ID, got, w)
		}
		e.assertLedger(bidder)
	}
}

// A huge min_increment must not wrap the minimum bid around int64 and let a
// lower bid replace the leader.
func TestCreateRejectsMoneyAboveCap(t *testing.T) {
	e := newAuctionEnv(t)
	seller := e.user(0)
	for name, mutate := range map[string]func(*NewAuction){
		"start":     func(n *NewAuction) { n.StartPrice = MaxMoney + 1 },
		"increment": func(n *NewAuction) { n.MinIncrement = math.MaxInt64 - 50_000 },
		"buy now":   func(n *NewAuction) { n.BuyNowPrice = ptr(int64(MaxMoney + 1)) },
	} {
		t.Run(name, func(t *testing.T) {
			in := sampleAuction()
			mutate(&in)
			if _, err := e.auctions.Create(context.Background(), seller, in); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
	if _, err := e.auctions.Bid(context.Background(), e.user(0), -1, MaxMoney+1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bid above cap err = %v", err)
	}
}
