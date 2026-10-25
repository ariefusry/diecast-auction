package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// wonAuction returns an auction that winner bought outright from seller.
func wonAuction(t *testing.T, e *auctionEnv, seller, winner int64) Auction {
	t.Helper()
	in := sampleAuction()
	in.BuyNowPrice = ptr(int64(300_000))
	a := e.active(seller, in)
	if _, err := e.auctions.BuyNow(context.Background(), winner, a.ID); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}
	return a
}

func TestChatBetweenSellerAndWinner(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	seller, winner, stranger := e.user(0), e.user(1_000_000), e.user(1_000_000)
	a := wonAuction(t, e, seller, winner)

	if _, err := e.auctions.Send(ctx, winner, a.ID, "  Halo, kirim ke Bandung ya  "); err != nil {
		t.Fatalf("winner Send: %v", err)
	}
	if _, err := e.auctions.Send(ctx, seller, a.ID, "Siap, resi menyusul"); err != nil {
		t.Fatalf("seller Send: %v", err)
	}
	got, err := e.auctions.Messages(ctx, seller, a.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("Messages = %+v, %v", got, err)
	}
	if got[0].SenderID != winner || got[0].Body != "Halo, kirim ke Bandung ya" || got[0].Sender == "" || got[1].SenderID != seller {
		t.Fatalf("messages out of order or untrimmed: %+v", got)
	}

	// Nobody else reads or writes the conversation.
	if _, err := e.auctions.Messages(ctx, stranger, a.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger reads: err = %v, want ErrForbidden", err)
	}
	if _, err := e.auctions.Send(ctx, stranger, a.ID, "hai"); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger writes: err = %v, want ErrForbidden", err)
	}
	// Before there is a winner there is nobody to talk to.
	open := e.active(seller, sampleAuction())
	if _, err := e.auctions.Send(ctx, seller, open.ID, "hai"); !errors.Is(err, ErrForbidden) {
		t.Errorf("open auction: err = %v, want ErrForbidden", err)
	}
	if _, err := e.auctions.Messages(ctx, seller, -1); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown auction: err = %v, want ErrNotFound", err)
	}
	for name, body := range map[string]string{"empty": "   ", "too long": strings.Repeat("a", 1001), "NUL": "a\x00b"} {
		if _, err := e.auctions.Send(ctx, winner, a.ID, body); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s body: err = %v, want ErrInvalidInput", name, err)
		}
	}
}

// A long conversation keeps its latest messages visible, still oldest first.
func TestMessagesKeepsNewestWhenCapped(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	seller, winner := e.user(0), e.user(1_000_000)
	a := wonAuction(t, e, seller, winner)
	if _, err := e.auctions.pool.Exec(ctx,
		"INSERT INTO messages (auction_id, sender_id, body) SELECT $1, $2, 'lama ' || n FROM generate_series(1, $3) n",
		a.ID, seller, chatLimit); err != nil {
		t.Fatal(err)
	}
	last, err := e.auctions.Send(ctx, winner, a.ID, "pesan terbaru")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	got, err := e.auctions.Messages(ctx, seller, a.ID)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(got) != chatLimit || got[len(got)-1].ID != last.ID || got[0].Body != "lama 2" {
		t.Fatalf("got %d messages, first %q, last id %d, want %d ending at %d", len(got), got[0].Body, got[len(got)-1].ID, chatLimit, last.ID)
	}
}

func TestSellerProfileIsPublic(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	sellerID, buyer := e.user(0), e.user(1_000_000)
	active := e.active(sellerID, sampleAuction())
	wonAuction(t, e, sellerID, buyer)
	e.draft(sellerID, sampleAuction()) // drafts stay private

	var username string
	if err := e.auctions.pool.QueryRow(ctx, "SELECT username FROM users WHERE id = $1", sellerID).Scan(&username); err != nil {
		t.Fatal(err)
	}
	p, err := e.auctions.Seller(ctx, username)
	if err != nil {
		t.Fatalf("Seller: %v", err)
	}
	if p.Username != username || p.Sold != 1 || len(p.Active) != 1 || p.Active[0].ID != active.ID || p.CreatedAt.IsZero() {
		t.Fatalf("Seller = %+v", p)
	}
	if active.Seller != username {
		t.Fatalf("auction seller = %q, want %q", active.Seller, username)
	}
	// Usernames are unique ignoring case, so a link typed in another case still lands here.
	if p, err := e.auctions.Seller(ctx, strings.ToUpper(username)); err != nil || p.Username != username {
		t.Fatalf("Seller(upper) = %q, %v; want %q", p.Username, err, username)
	}
	if _, err := e.auctions.Seller(ctx, "nobody_"+username); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown seller: err = %v, want ErrNotFound", err)
	}
}

func TestMineTracksMyHighestBid(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	seller, me, rival := e.user(0), e.user(2_000_000), e.user(2_000_000)
	a := e.active(seller, sampleAuction())
	for _, b := range []struct{ who, amount int64 }{{me, 100_000}, {rival, 110_000}, {me, 120_000}, {rival, 130_000}} {
		if _, err := e.auctions.Bid(ctx, b.who, a.ID, b.amount); err != nil {
			t.Fatalf("Bid: %v", err)
		}
	}
	mine, err := e.auctions.Mine(ctx, me)
	if err != nil {
		t.Fatalf("Mine: %v", err)
	}
	if got := mine.MyBids[a.ID]; got != 120_000 {
		t.Fatalf("my highest bid = %d, want 120000 (%+v)", got, mine.MyBids)
	}
}

func TestConversationsListsChatsWithUnreadCounts(t *testing.T) {
	e := newAuctionEnv(t)
	ctx := context.Background()
	seller, winner, stranger := e.user(0), e.user(1_000_000), e.user(1_000_000)
	older := wonAuction(t, e, seller, winner)
	newer := wonAuction(t, e, seller, winner)
	e.active(seller, sampleAuction()) // no winner: no conversation

	for _, body := range []string{"halo", "sudah dikirim?"} {
		if _, err := e.auctions.Send(ctx, winner, older.ID, body); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := e.auctions.Send(ctx, seller, older.ID, "besok ya")
	if err != nil {
		t.Fatal(err)
	}

	got, err := e.auctions.Conversations(ctx, seller)
	if err != nil {
		t.Fatalf("Conversations: %v", err)
	}
	if len(got) != 2 || got[0].AuctionID != older.ID || got[1].AuctionID != newer.ID {
		t.Fatalf("want the chat with messages first, then the silent one: %+v", got)
	}
	c := got[0]
	// The seller's own reply is the last message but not unread for them.
	if c.Unread != 2 || c.LastBody != "besok ya" || !c.LastMine || !c.Selling || c.Price != 300_000 || c.Other == "" {
		t.Fatalf("seller's view = %+v", c)
	}
	if mine, _ := e.auctions.Conversations(ctx, winner); len(mine) != 2 || mine[0].Unread != 1 || mine[0].Selling {
		t.Fatalf("winner's view = %+v", mine)
	}

	if err := e.auctions.MarkRead(ctx, seller, older.ID, reply.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	// A late, older marker must not bring messages back to unread.
	if err := e.auctions.MarkRead(ctx, seller, older.ID, 1); err != nil {
		t.Fatalf("MarkRead older: %v", err)
	}
	if got, _ := e.auctions.Conversations(ctx, seller); got[0].Unread != 0 {
		t.Fatalf("after read: unread = %d", got[0].Unread)
	}
	if err := e.auctions.MarkRead(ctx, stranger, older.ID, reply.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("stranger MarkRead err = %v, want ErrForbidden", err)
	}
	if none, _ := e.auctions.Conversations(ctx, stranger); len(none) != 0 {
		t.Fatalf("stranger sees %+v", none)
	}
}
