package service

import (
	"context"
	"testing"
	"time"
)

func (e *auctionEnv) closeExpired() {
	e.t.Helper()
	if _, err := e.auctions.CloseExpired(context.Background()); err != nil {
		e.t.Fatalf("CloseExpired: %v", err)
	}
}

func TestCloseExpiredSettlesWinnerToSeller(t *testing.T) {
	e := newAuctionEnv(t)
	seller, alice, bob := e.user(0), e.user(500_000), e.user(500_000)
	ctx := context.Background()
	a := e.active(seller, sampleAuction())
	e.auctions.Bid(ctx, alice, a.ID, 100_000)
	if _, err := e.auctions.Bid(ctx, bob, a.ID, 120_000); err != nil {
		t.Fatalf("Bid: %v", err)
	}
	e.setEndTime(a.ID, "-1 second")

	e.closeExpired()
	got, _ := e.auctions.Get(ctx, a.ID)
	if got.Status != "completed" || got.WinnerID == nil || *got.WinnerID != bob {
		t.Fatalf("after close = %+v", got)
	}
	if w := e.wallet(bob); w != (Wallet{Available: 380_000}) {
		t.Fatalf("winner wallet = %+v", w)
	}
	if w := e.wallet(seller); w != (Wallet{Available: 120_000}) {
		t.Fatalf("seller wallet = %+v", w)
	}
	if w := e.wallet(alice); w != (Wallet{Available: 500_000}) {
		t.Fatalf("loser wallet = %+v", w)
	}
	e.assertLedger(seller, alice, bob)

	// A second pass must not settle again.
	e.closeExpired()
	if w := e.wallet(seller); w.Available != 120_000 {
		t.Fatalf("seller wallet after second pass = %+v", w)
	}
}

func TestCloseExpiredWithoutBidsHasNoWinner(t *testing.T) {
	e := newAuctionEnv(t)
	seller := e.user(0)
	ctx := context.Background()
	expired := e.active(seller, sampleAuction())
	e.setEndTime(expired.ID, "-1 second")
	running := e.active(seller, sampleAuction())
	draft := e.draft(seller, sampleAuction())

	e.closeExpired()
	if got, _ := e.auctions.Get(ctx, expired.ID); got.Status != "completed" || got.WinnerID != nil {
		t.Fatalf("expired = %+v", got)
	}
	if got, _ := e.auctions.Get(ctx, running.ID); got.Status != "active" {
		t.Fatalf("running = %+v", got)
	}
	if got, _ := e.auctions.Get(ctx, draft.ID); got.Status != "draft" {
		t.Fatalf("draft = %+v", got)
	}
	e.assertLedger(seller)
}

// Several workers (or API instances) closing at once must settle each auction exactly once.
func TestConcurrentCloseSettlesOnce(t *testing.T) {
	e := newAuctionEnv(t)
	seller, bidder := e.user(0), e.user(5_000_000)
	ctx := context.Background()
	for range 5 {
		a := e.active(seller, sampleAuction())
		if _, err := e.auctions.Bid(ctx, bidder, a.ID, 100_000); err != nil {
			t.Fatalf("Bid: %v", err)
		}
		e.setEndTime(a.ID, "-1 second")
	}

	var fns []func() error
	for range 4 {
		fns = append(fns, func() error { _, err := e.auctions.CloseExpired(ctx); return err })
	}
	race(t, fns...)
	if w := e.wallet(seller); w.Available != 500_000 {
		t.Fatalf("seller wallet = %+v, want 500000", w)
	}
	if w := e.wallet(bidder); w != (Wallet{Available: 4_500_000}) {
		t.Fatalf("bidder wallet = %+v", w)
	}
	e.assertLedger(seller, bidder)
}

// A last-moment bid and the closer race on the same auction. Either the bid
// lands (and extends the auction) or the close wins with the old leader; never both.
func TestCloseRacesLastBid(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	for i := range 10 {
		seller, alice, bob := e.user(0), e.user(500_000), e.user(500_000)
		a := e.active(seller, sampleAuction())
		if _, err := e.auctions.Bid(ctx, alice, a.ID, 100_000); err != nil {
			t.Fatalf("Bid: %v", err)
		}
		e.setEndTime(a.ID, "50 milliseconds")
		race(t,
			func() error {
				time.Sleep(time.Duration(i*10) * time.Millisecond)
				_, err := e.auctions.Bid(ctx, bob, a.ID, 110_000)
				return err
			},
			func() error {
				time.Sleep(60 * time.Millisecond)
				_, err := e.auctions.CloseExpired(ctx)
				return err
			},
		)
		got, _ := e.auctions.Get(ctx, a.ID)
		closed := got.Status == "completed" && *got.WinnerID == alice &&
			e.wallet(seller).Available == 100_000 && e.wallet(bob) == (Wallet{Available: 500_000})
		outbid := got.Status == "active" && *got.HighestBidderID == bob &&
			e.wallet(seller).Available == 0 && e.wallet(bob).Locked == 110_000
		if closed == outbid {
			t.Fatalf("round %d mixed state %+v", i, got)
		}
		e.assertLedger(seller, alice, bob)
	}
}

// One auction that cannot settle (here: escrow already gone) must not block
// the others behind it.
func TestCloseExpiredSkipsAuctionThatFails(t *testing.T) {
	e := newAuctionEnv(t)
	seller, alice := e.user(0), e.user(500_000)
	ctx := context.Background()
	broken := e.active(seller, sampleAuction())
	if _, err := e.auctions.Bid(ctx, alice, broken.ID, 100_000); err != nil {
		t.Fatalf("Bid: %v", err)
	}
	e.auctions.pool.Exec(ctx, "UPDATE wallets SET locked_balance = 0 WHERE user_id = $1", alice)
	e.setEndTime(broken.ID, "-1 hour")
	t.Cleanup(func() {
		e.auctions.pool.Exec(ctx, "UPDATE auctions SET status = 'cancelled' WHERE id = $1", broken.ID)
	})
	healthy := e.active(seller, sampleAuction())
	e.setEndTime(healthy.ID, "-1 second")

	if _, err := e.auctions.CloseExpired(ctx); err == nil {
		t.Fatal("CloseExpired hid the failing auction")
	}
	if got, _ := e.auctions.Get(ctx, healthy.ID); got.Status != "completed" {
		t.Fatalf("healthy auction blocked: %+v", got)
	}
	if got, _ := e.auctions.Get(ctx, broken.ID); got.Status != "active" {
		t.Fatalf("broken auction = %+v, want rolled back to active", got)
	}
}
