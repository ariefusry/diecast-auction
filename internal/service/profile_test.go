package service

import (
	"context"
	"errors"
	"testing"
)

func TestProfile(t *testing.T) {
	a := newAuth(t)
	user, email := registerUser(t, a)
	p, err := a.Profile(context.Background(), user.ID)
	if err != nil || p.ID != user.ID || p.Username != user.Username || p.Email != email || p.CreatedAt.IsZero() {
		t.Fatalf("Profile = %+v, %v", p, err)
	}
	if _, err := a.Profile(context.Background(), -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user err = %v, want ErrNotFound", err)
	}
}

func TestMineSortsAuctionsByRole(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	seller, me, rival := e.user(0), e.user(2_000_000), e.user(2_000_000)
	bid := func(who, id, amount int64) {
		t.Helper()
		if _, err := e.auctions.Bid(ctx, who, id, amount); err != nil {
			t.Fatalf("Bid: %v", err)
		}
	}

	open := e.active(seller, sampleAuction()) // I bid and still lead
	bid(me, open.ID, 100_000)

	withBuyNow := sampleAuction()
	withBuyNow.BuyNowPrice = ptr(int64(300_000))
	bought := e.active(seller, withBuyNow) // I buy it outright
	if _, err := e.auctions.BuyNow(ctx, me, bought.ID); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}

	lost := e.active(seller, sampleAuction()) // the rival outbids me and wins
	bid(me, lost.ID, 100_000)
	bid(rival, lost.ID, 110_000)
	e.setEndTime(lost.ID, "-1 second")
	if _, err := e.auctions.CloseExpired(ctx); err != nil {
		t.Fatalf("CloseExpired: %v", err)
	}

	mine, err := e.auctions.Mine(ctx, me)
	if err != nil {
		t.Fatalf("Mine: %v", err)
	}
	ids := func(as []Auction) []int64 {
		out := []int64{}
		for _, a := range as {
			out = append(out, a.ID)
		}
		return out
	}
	if got := ids(mine.Won); len(got) != 1 || got[0] != bought.ID {
		t.Errorf("won = %v, want [%d]", got, bought.ID)
	}
	if got := ids(mine.Bidding); len(got) != 1 || got[0] != open.ID {
		t.Errorf("bidding = %v, want [%d]", got, open.ID)
	}
	if got := ids(mine.Lost); len(got) != 1 || got[0] != lost.ID {
		t.Errorf("lost = %v, want [%d]", got, lost.ID)
	}
	if len(mine.Selling) != 0 {
		t.Errorf("selling = %v, want none", ids(mine.Selling))
	}

	theirs, err := e.auctions.Mine(ctx, seller)
	if err != nil || len(theirs.Selling) != 3 || len(theirs.Won)+len(theirs.Bidding)+len(theirs.Lost) != 0 {
		t.Fatalf("seller's Mine = %+v, %v", theirs, err)
	}
}

func TestWalletHistoryShowsWhichWayMoneyMoved(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	seller, me, rival := e.user(0), e.user(1_000_000), e.user(1_000_000)

	a := e.active(seller, sampleAuction())
	if _, err := e.auctions.Bid(ctx, me, a.ID, 100_000); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auctions.Bid(ctx, rival, a.ID, 110_000); err != nil {
		t.Fatal(err)
	}
	withBuyNow := sampleAuction()
	withBuyNow.BuyNowPrice = ptr(int64(300_000))
	b := e.active(seller, withBuyNow)
	if _, err := e.auctions.BuyNow(ctx, me, b.ID); err != nil {
		t.Fatal(err)
	}

	got, err := e.wallets.History(ctx, me)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	// Newest first: buy-now paid, outbid refund, bid escrow, the deposit.
	want := []struct {
		typ, dir string
		amount   int64
	}{{"buy_now", "out", 300_000}, {"release", "in", 100_000}, {"lock", "hold", 100_000}, {"deposit", "in", 1_000_000}}
	if len(got) != len(want) {
		t.Fatalf("History = %+v", got)
	}
	for i, w := range want {
		if got[i].Type != w.typ || got[i].Direction != w.dir || got[i].Amount != w.amount {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
		}
	}
	if got[0].AuctionID == nil || *got[0].AuctionID != b.ID || got[0].Title == "" {
		t.Errorf("buy_now entry lacks its auction: %+v", got[0])
	}

	sold, err := e.wallets.History(ctx, seller)
	if err != nil || len(sold) != 1 || sold[0].Type != "buy_now" || sold[0].Direction != "in" {
		t.Fatalf("seller History = %+v, %v", sold, err)
	}
}
