package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxDurationMinutes caps an auction at 30 days.
const MaxDurationMinutes = 30 * 24 * 60

type Item struct {
	ID          int64    `json:"id,omitempty"`
	Brand       string   `json:"brand"`
	ModelName   string   `json:"model_name"`
	Series      string   `json:"series"`
	Year        int      `json:"year"`
	Color       string   `json:"color"`
	Tags        []string `json:"tags"`
	Condition   string   `json:"condition,omitempty"`
	ImageURL    string   `json:"image_url"`
	SourceURL   string   `json:"source_url"` // attribution for wiki-sourced items
	Description string   `json:"description"`
}

type NewAuction struct {
	Item            Item   `json:"item"`
	StartPrice      int64  `json:"start_price"`
	MinIncrement    int64  `json:"min_increment"`
	BuyNowPrice     *int64 `json:"buy_now_price"`
	DurationMinutes int    `json:"duration_minutes"`
}

type Auction struct {
	ID                int64      `json:"id"`
	SellerID          int64      `json:"seller_id"`
	Item              Item       `json:"item"`
	StartPrice        int64      `json:"start_price"`
	MinIncrement      int64      `json:"min_increment"`
	BuyNowPrice       *int64     `json:"buy_now_price"`
	CurrentHighestBid *int64     `json:"current_highest_bid"`
	HighestBidderID   *int64     `json:"highest_bidder_id"`
	Status            string     `json:"status"`
	DurationMinutes   int        `json:"duration_minutes"`
	StartTime         *time.Time `json:"start_time"`
	EndTime           *time.Time `json:"end_time"`
	WinnerID          *int64     `json:"winner_id"`
}

// Auctions owns the auction lifecycle and every wallet movement it causes.
//
// Every mutating method runs one transaction with a fixed lock order: the
// auction row first (serializing all writes to one auction), then the
// wallets it touches by ascending user_id. Time comes from the database now().
//
// Ledger direction for buy_now and settle rows: the auction's seller is
// credited, anyone else is debited. This is unambiguous because a seller can
// never bid on or buy their own auction.
type Auctions struct {
	pool *pgxpool.Pool
}

func NewAuctions(pool *pgxpool.Pool) *Auctions {
	return &Auctions{pool: pool}
}

// Create stores the item and a draft auction for it.
func (s *Auctions) Create(ctx context.Context, sellerID int64, in NewAuction) (Auction, error) {
	it := in.Item
	it.Brand, it.ModelName = strings.TrimSpace(it.Brand), strings.TrimSpace(it.ModelName)
	if it.Brand == "" || it.ModelName == "" {
		return Auction{}, fmt.Errorf("%w: brand and model_name are required", ErrInvalidInput)
	}
	if in.DurationMinutes < 1 || in.DurationMinutes > MaxDurationMinutes {
		return Auction{}, fmt.Errorf("%w: duration_minutes must be 1-%d", ErrInvalidInput, MaxDurationMinutes)
	}
	if it.Tags == nil {
		it.Tags = []string{}
	}

	var id int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var itemID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO items (seller_id, brand, model_name, series, year, color, tags, condition, image_url, source_url, description)
			 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, 0), NULLIF($6, ''), $7, $8, NULLIF($9, ''), NULLIF($10, ''), $11)
			 RETURNING id`,
			sellerID, it.Brand, it.ModelName, it.Series, it.Year, it.Color, it.Tags, it.Condition, it.ImageURL, it.SourceURL, it.Description).
			Scan(&itemID)
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO auctions (item_id, start_price, min_increment, buy_now_price, duration)
			 VALUES ($1, $2, $3, $4, make_interval(mins => $5)) RETURNING id`,
			itemID, in.StartPrice, in.MinIncrement, in.BuyNowPrice, in.DurationMinutes).Scan(&id)
	})
	// CHECK constraints own the remaining rules (prices, buy-now > start, condition).
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return Auction{}, fmt.Errorf("%w: violates %s", ErrInvalidInput, pgErr.ConstraintName)
	}
	if err != nil {
		return Auction{}, err
	}
	return s.Get(ctx, id)
}

func (s *Auctions) Get(ctx context.Context, id int64) (Auction, error) {
	var a Auction
	err := s.pool.QueryRow(ctx,
		`SELECT a.id, i.seller_id, i.id, i.brand, i.model_name, coalesce(i.series, ''), coalesce(i.year, 0),
		        coalesce(i.color, ''), i.tags, i.condition, coalesce(i.image_url, ''), coalesce(i.source_url, ''), i.description,
		        a.start_price, a.min_increment, a.buy_now_price, a.current_highest_bid, a.highest_bidder_id,
		        a.status, extract(epoch FROM a.duration)::int / 60, a.start_time, a.end_time, a.winner_id
		 FROM auctions a JOIN items i ON i.id = a.item_id WHERE a.id = $1`, id).
		Scan(&a.ID, &a.SellerID, &a.Item.ID, &a.Item.Brand, &a.Item.ModelName, &a.Item.Series, &a.Item.Year,
			&a.Item.Color, &a.Item.Tags, &a.Item.Condition, &a.Item.ImageURL, &a.Item.SourceURL, &a.Item.Description,
			&a.StartPrice, &a.MinIncrement, &a.BuyNowPrice, &a.CurrentHighestBid, &a.HighestBidderID,
			&a.Status, &a.DurationMinutes, &a.StartTime, &a.EndTime, &a.WinnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Auction{}, ErrNotFound
	}
	return a, err
}

// Publish moves a draft to active with end_time = now() + duration.
func (s *Auctions) Publish(ctx context.Context, userID, id int64) (Auction, error) {
	return s.mutate(ctx, id, func(tx pgx.Tx, l locked) error {
		if l.sellerID != userID {
			return fmt.Errorf("%w: only the seller can publish", ErrForbidden)
		}
		if l.status != "draft" {
			return fmt.Errorf("%w: auction is %s, not draft", ErrConflict, l.status)
		}
		_, err := tx.Exec(ctx,
			`UPDATE auctions SET status = 'active', start_time = now(), end_time = now() + duration WHERE id = $1`, id)
		return err
	})
}

// Cancel is allowed for a draft or active auction that has no bids, so no escrow needs releasing.
func (s *Auctions) Cancel(ctx context.Context, userID, id int64) (Auction, error) {
	return s.mutate(ctx, id, func(tx pgx.Tx, l locked) error {
		if l.sellerID != userID {
			return fmt.Errorf("%w: only the seller can cancel", ErrForbidden)
		}
		if l.status != "draft" && l.status != "active" {
			return fmt.Errorf("%w: auction is %s", ErrConflict, l.status)
		}
		if l.highestBidderID != nil {
			return fmt.Errorf("%w: auction already has bids", ErrConflict)
		}
		_, err := tx.Exec(ctx, "UPDATE auctions SET status = 'cancelled' WHERE id = $1", id)
		return err
	})
}

// Bid escrows amount from the bidder, releases the previous leader's escrow,
// and extends end_time by 2 minutes when the bid lands in the final 2 minutes.
func (s *Auctions) Bid(ctx context.Context, bidderID, id, amount int64) (Auction, error) {
	if amount <= 0 {
		return Auction{}, fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	return s.mutate(ctx, id, func(tx pgx.Tx, l locked) error {
		if err := l.open(); err != nil {
			return err
		}
		if l.sellerID == bidderID {
			return fmt.Errorf("%w: sellers cannot bid on their own auction", ErrForbidden)
		}
		minBid := l.startPrice
		if l.highestBidderID != nil {
			if *l.highestBidderID == bidderID {
				return fmt.Errorf("%w: you are already the highest bidder", ErrConflict)
			}
			minBid = *l.highestBid + l.minIncrement
		}
		if amount < minBid {
			return fmt.Errorf("%w: bid must be at least %d", ErrConflict, minBid)
		}

		ids := []int64{bidderID}
		if l.highestBidderID != nil {
			ids = append(ids, *l.highestBidderID)
		}
		available, err := lockWallets(ctx, tx, ids)
		if err != nil {
			return err
		}
		if available[bidderID] < amount {
			return fmt.Errorf("%w: insufficient balance", ErrConflict)
		}
		if err := move(ctx, tx, bidderID, id, "lock", amount, -amount, amount); err != nil {
			return err
		}
		if l.highestBidderID != nil {
			prev := *l.highestBid
			if err := move(ctx, tx, *l.highestBidderID, id, "release", prev, prev, -prev); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO bids (auction_id, bidder_id, amount) VALUES ($1, $2, $3)", id, bidderID, amount); err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE auctions SET current_highest_bid = $2, highest_bidder_id = $3,
			   end_time = CASE WHEN end_time - now() <= interval '2 minutes'
			                   THEN end_time + interval '2 minutes' ELSE end_time END
			 WHERE id = $1`, id, amount, bidderID)
		return err
	})
}

// BuyNow pays buy_now_price straight from buyer to seller and completes the
// auction. Only allowed before the first bid, so there is no escrow to refund.
func (s *Auctions) BuyNow(ctx context.Context, buyerID, id int64) (Auction, error) {
	return s.mutate(ctx, id, func(tx pgx.Tx, l locked) error {
		if err := l.open(); err != nil {
			return err
		}
		if l.sellerID == buyerID {
			return fmt.Errorf("%w: sellers cannot buy their own auction", ErrForbidden)
		}
		if l.buyNowPrice == nil {
			return fmt.Errorf("%w: auction has no buy-now price", ErrConflict)
		}
		if l.highestBidderID != nil {
			return fmt.Errorf("%w: buy-now is closed once bidding starts", ErrConflict)
		}
		price := *l.buyNowPrice
		available, err := lockWallets(ctx, tx, []int64{buyerID, l.sellerID})
		if err != nil {
			return err
		}
		if available[buyerID] < price {
			return fmt.Errorf("%w: insufficient balance", ErrConflict)
		}
		if err := move(ctx, tx, buyerID, id, "buy_now", price, -price, 0); err != nil {
			return err
		}
		if err := move(ctx, tx, l.sellerID, id, "buy_now", price, price, 0); err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			"UPDATE auctions SET status = 'completed', winner_id = $2, end_time = now() WHERE id = $1", id, buyerID)
		return err
	})
}

// CloseExpired completes every active auction past end_time, one transaction
// each, and returns how many it closed. A winner's escrow moves to the seller
// (ledger settle). SKIP LOCKED lets several instances run this at once without
// settling twice, and skips auctions a bid is holding; the next pass gets them.
// An auction that fails to settle is rolled back and skipped for the rest of
// the pass so it cannot block the ones behind it; its error is returned.
func (s *Auctions) CloseExpired(ctx context.Context) (int, error) {
	var n int
	failed := []int64{} // not nil: pgx sends nil as NULL, and id <> ALL(NULL) matches nothing
	var errs error
	for {
		var id int64
		err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			var sellerID int64
			var bid, winner *int64
			err := tx.QueryRow(ctx,
				`SELECT a.id, i.seller_id, a.current_highest_bid, a.highest_bidder_id
				 FROM auctions a JOIN items i ON i.id = a.item_id
				 WHERE a.status = 'active' AND a.end_time <= now() AND a.id <> ALL($1)
				 ORDER BY a.end_time LIMIT 1
				 FOR UPDATE OF a SKIP LOCKED`, failed).Scan(&id, &sellerID, &bid, &winner)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if winner != nil {
				if _, err := lockWallets(ctx, tx, []int64{*winner, sellerID}); err != nil {
					return err
				}
				if err := move(ctx, tx, *winner, id, "settle", *bid, 0, -*bid); err != nil {
					return err
				}
				if err := move(ctx, tx, sellerID, id, "settle", *bid, *bid, 0); err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, "UPDATE auctions SET status = 'completed', winner_id = $2 WHERE id = $1", id, winner)
			return err
		})
		switch {
		case err != nil && id != 0 && ctx.Err() == nil:
			failed = append(failed, id)
			errs = errors.Join(errs, fmt.Errorf("settle auction %d: %w", id, err))
		case err != nil:
			return n, errors.Join(errs, err)
		case id == 0:
			return n, errs
		default:
			n++
		}
	}
}

// locked is the auction state read under its row lock.
type locked struct {
	sellerID        int64
	status          string
	startPrice      int64
	minIncrement    int64
	buyNowPrice     *int64
	highestBid      *int64
	highestBidderID *int64
	ended           bool
}

func (l locked) open() error {
	if l.status != "active" || l.ended {
		return fmt.Errorf("%w: auction is not open", ErrConflict)
	}
	return nil
}

// mutate locks the auction row, runs fn in the same transaction, and returns the fresh auction.
// now() is the transaction start, so a bid that waits on the lock is judged at
// its arrival time, for both the end check and the anti-sniping window. Intentional.
func (s *Auctions) mutate(ctx context.Context, id int64, fn func(pgx.Tx, locked) error) (Auction, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var l locked
		err := tx.QueryRow(ctx,
			`SELECT i.seller_id, a.status, a.start_price, a.min_increment, a.buy_now_price,
			        a.current_highest_bid, a.highest_bidder_id, coalesce(a.end_time <= now(), false)
			 FROM auctions a JOIN items i ON i.id = a.item_id
			 WHERE a.id = $1 FOR UPDATE OF a`, id).
			Scan(&l.sellerID, &l.status, &l.startPrice, &l.minIncrement, &l.buyNowPrice,
				&l.highestBid, &l.highestBidderID, &l.ended)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return fn(tx, l)
	})
	if err != nil {
		return Auction{}, err
	}
	return s.Get(ctx, id)
}

// lockWallets locks the given wallets in ascending user_id order (ORDER BY runs
// before FOR UPDATE takes the locks) and returns their available balances.
func lockWallets(ctx context.Context, tx pgx.Tx, userIDs []int64) (map[int64]int64, error) {
	rows, err := tx.Query(ctx,
		"SELECT user_id, available_balance FROM wallets WHERE user_id = ANY($1) ORDER BY user_id FOR UPDATE", userIDs)
	if err != nil {
		return nil, err
	}
	available := map[int64]int64{}
	var id, bal int64
	_, err = pgx.ForEachRow(rows, []any{&id, &bal}, func() error {
		available[id] = bal
		return nil
	})
	return available, err
}

// move applies balance deltas to one wallet and writes the matching ledger row.
func move(ctx context.Context, tx pgx.Tx, userID, auctionID int64, typ string, amount, dAvailable, dLocked int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE wallets SET available_balance = available_balance + $2, locked_balance = locked_balance + $3
		 WHERE user_id = $1`, userID, dAvailable, dLocked)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		"INSERT INTO wallet_transactions (user_id, type, amount, auction_id) VALUES ($1, $2, $3, $4)",
		userID, typ, amount, auctionID)
	return err
}
