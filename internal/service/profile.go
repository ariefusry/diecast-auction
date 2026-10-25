package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Profile is what a signed-in user sees about themselves.
type Profile struct {
	User
	CreatedAt time.Time `json:"created_at"`
}

func (a *Auth) Profile(ctx context.Context, userID int64) (Profile, error) {
	var p Profile
	err := a.pool.QueryRow(ctx, "SELECT id, email, username, created_at FROM users WHERE id = $1", userID).
		Scan(&p.ID, &p.Email, &p.Username, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}

// MyAuctions groups a user's auctions by their part in them.
type MyAuctions struct {
	Won     []Auction `json:"won"`     // won by bidding or bought outright
	Bidding []Auction `json:"bidding"` // still open, with at least one of my bids
	Lost    []Auction `json:"lost"`    // closed, I bid, someone else won
	Selling []Auction `json:"selling"` // listed by me, any status
	// MyBids is my highest bid per auction in Bidding and Lost, for the tracker.
	MyBids map[int64]int64 `json:"my_bids"`
}

// myLimit caps each list; a profile shows recent activity, not an archive.
const myLimit = 50

func (s *Auctions) Mine(ctx context.Context, userID int64) (MyAuctions, error) {
	const iBid = "EXISTS (SELECT 1 FROM bids b WHERE b.auction_id = a.id AND b.bidder_id = $1)"
	var out MyAuctions
	for _, q := range []struct {
		dst   *[]Auction
		where string
	}{
		{&out.Won, "a.winner_id = $1 ORDER BY a.end_time DESC"},
		{&out.Bidding, "a.status = 'active' AND " + iBid + " ORDER BY a.end_time"},
		{&out.Lost, "a.status = 'completed' AND a.winner_id IS DISTINCT FROM $1 AND " + iBid + " ORDER BY a.end_time DESC"},
		{&out.Selling, "i.seller_id = $1 ORDER BY a.id DESC"},
	} {
		rows, err := s.pool.Query(ctx, auctionSelect+" WHERE "+q.where+" LIMIT $2", userID, myLimit)
		if err != nil {
			return MyAuctions{}, err
		}
		list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Auction, error) { return scanAuction(r) })
		if err != nil {
			return MyAuctions{}, err
		}
		*q.dst = list
	}

	out.MyBids = map[int64]int64{}
	var ids []int64
	for _, a := range append(out.Bidding, out.Lost...) {
		ids = append(ids, a.ID)
	}
	rows, err := s.pool.Query(ctx,
		"SELECT auction_id, max(amount) FROM bids WHERE bidder_id = $1 AND auction_id = ANY($2) GROUP BY auction_id", userID, ids)
	if err != nil {
		return MyAuctions{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, amount int64
		if err := rows.Scan(&id, &amount); err != nil {
			return MyAuctions{}, err
		}
		out.MyBids[id] = amount
	}
	return out, rows.Err()
}

// SellerProfile is the public page of anyone who lists auctions.
type SellerProfile struct {
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	Sold      int       `json:"sold"`   // auctions that ended with a buyer
	Active    []Auction `json:"active"` // open now, closing soonest first
}

func (s *Auctions) Seller(ctx context.Context, username string) (SellerProfile, error) {
	p := SellerProfile{Username: username}
	var id int64
	err := s.pool.QueryRow(ctx, "SELECT id, created_at FROM users WHERE username = $1", username).Scan(&id, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SellerProfile{}, ErrNotFound
	}
	if err != nil {
		return SellerProfile{}, err
	}
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM auctions a JOIN items i ON i.id = a.item_id
		WHERE i.seller_id = $1 AND a.status = 'completed' AND a.winner_id IS NOT NULL`, id).Scan(&p.Sold)
	if err != nil {
		return SellerProfile{}, err
	}
	rows, err := s.pool.Query(ctx, auctionSelect+" WHERE i.seller_id = $1 AND a.status = 'active' ORDER BY a.end_time LIMIT $2", id, myLimit)
	if err != nil {
		return SellerProfile{}, err
	}
	p.Active, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Auction, error) { return scanAuction(r) })
	return p, err
}

// LedgerEntry is one wallet_transactions row as the wallet's owner reads it.
type LedgerEntry struct {
	Type      string    `json:"type" enums:"deposit,lock,release,settle,buy_now"`
	Direction string    `json:"direction" enums:"in,out,hold"` // in: spendable money arrived; out: money left; hold: locked for a bid
	Amount    int64     `json:"amount"`
	AuctionID *int64    `json:"auction_id"`
	Title     string    `json:"title"` // the auction's item, empty for deposits
	CreatedAt time.Time `json:"created_at"`
}

// History returns the user's latest wallet movements, newest first.
func (w *Wallets) History(ctx context.Context, userID int64) ([]LedgerEntry, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT t.type, t.amount, t.auction_id, coalesce(i.model_name, ''), coalesce(i.seller_id = t.user_id, false), t.created_at
		FROM wallet_transactions t
		LEFT JOIN auctions a ON a.id = t.auction_id
		LEFT JOIN items i ON i.id = a.item_id
		WHERE t.user_id = $1 ORDER BY t.id DESC LIMIT $2`, userID, myLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (LedgerEntry, error) {
		var e LedgerEntry
		var seller bool
		err := r.Scan(&e.Type, &e.Amount, &e.AuctionID, &e.Title, &seller, &e.CreatedAt)
		// Same rule as the ledger itself: on settle and buy_now the seller is
		// credited and anyone else paid.
		switch {
		case e.Type == "lock":
			e.Direction = "hold"
		case e.Type == "deposit", e.Type == "release", seller:
			e.Direction = "in"
		default:
			e.Direction = "out"
		}
		return e, err
	})
}
