package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxDurationMinutes caps an auction at 30 days.
const MaxDurationMinutes = 30 * 24 * 60

// MaxMoney caps every price and bid (Rp1 triliun). Far above any diecast, and
// small enough that bid + increment can never overflow int64.
const MaxMoney int64 = 1_000_000_000_000

type Item struct {
	ID          int64    `json:"id,omitempty"`
	Category    string   `json:"category" enums:"diecast,diorama"` // empty means diecast
	Brand       string   `json:"brand"`                            // diecast only
	Artist      string   `json:"artist,omitempty"`                 // diorama only: who builds it
	Size        string   `json:"size,omitempty" example:"30 x 20 cm"`
	ReadyWeeks  int      `json:"ready_weeks,omitempty"` // diorama PO estimate; 0 means finished
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
	Seller            string     `json:"seller"` // seller's username, public like the listing
	Item              Item       `json:"item"`
	StartPrice        int64      `json:"start_price"`
	MinIncrement      int64      `json:"min_increment"`
	BuyNowPrice       *int64     `json:"buy_now_price"`
	CurrentHighestBid *int64     `json:"current_highest_bid"`
	Status            string     `json:"status"`
	DurationMinutes   int        `json:"duration_minutes"`
	StartTime         *time.Time `json:"start_time"`
	EndTime           *time.Time `json:"end_time"`
	Sold              bool       `json:"sold"` // completed with a winner
	// Bidder and winner IDs never leave the server: next to a seller's public
	// username they would unmask the bid history. ForViewer turns them into booleans.
	HighestBidderID *int64 `json:"-"`
	WinnerID        *int64 `json:"-"`
	Leading         bool   `json:"leading"` // the viewer holds the highest bid
	Won             bool   `json:"won"`     // the viewer won
}

// ForViewer fills the viewer-relative fields for userID; 0 is anonymous.
func (a Auction) ForViewer(userID int64) Auction {
	a.Leading = userID != 0 && a.HighestBidderID != nil && *a.HighestBidderID == userID
	a.Won = userID != 0 && a.WinnerID != nil && *a.WinnerID == userID
	return a
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
	it, err := validItem(in.Item)
	if err != nil {
		return Auction{}, err
	}
	if in.DurationMinutes < 1 || in.DurationMinutes > MaxDurationMinutes {
		return Auction{}, fmt.Errorf("%w: duration_minutes must be 1-%d", ErrInvalidInput, MaxDurationMinutes)
	}
	if in.StartPrice > MaxMoney || in.MinIncrement > MaxMoney || in.BuyNowPrice != nil && *in.BuyNowPrice > MaxMoney {
		return Auction{}, fmt.Errorf("%w: prices must be at most %d", ErrInvalidInput, MaxMoney)
	}

	var id int64
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var itemID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO items (seller_id, brand, model_name, series, year, color, tags, condition, image_url, source_url, description,
			                    category, artist, size, ready_weeks)
			 VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''), NULLIF($5, 0), NULLIF($6, ''), $7, NULLIF($8, ''), NULLIF($9, ''),
			         NULLIF($10, ''), $11, $12, NULLIF($13, ''), NULLIF($14, ''), NULLIF($15, 0))
			 RETURNING id`,
			sellerID, it.Brand, it.ModelName, it.Series, it.Year, it.Color, it.Tags, it.Condition, it.ImageURL, it.SourceURL, it.Description,
			it.Category, it.Artist, it.Size, it.ReadyWeeks).
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

// auctionSelect lists the columns scanAuction reads, in order.
const auctionSelect = `SELECT a.id, i.seller_id, i.id, coalesce(i.brand, ''), i.model_name, coalesce(i.series, ''), coalesce(i.year, 0),
	coalesce(i.color, ''), i.tags, coalesce(i.condition, ''), coalesce(i.image_url, ''), coalesce(i.source_url, ''), i.description,
	i.category, coalesce(i.artist, ''), coalesce(i.size, ''), coalesce(i.ready_weeks, 0), su.username,
	a.start_price, a.min_increment, a.buy_now_price, a.current_highest_bid, a.highest_bidder_id,
	a.status, extract(epoch FROM a.duration)::int / 60, a.start_time, a.end_time, a.winner_id
	FROM auctions a JOIN items i ON i.id = a.item_id JOIN users su ON su.id = i.seller_id`

func scanAuction(row pgx.Row) (Auction, error) {
	var a Auction
	err := row.Scan(&a.ID, &a.SellerID, &a.Item.ID, &a.Item.Brand, &a.Item.ModelName, &a.Item.Series, &a.Item.Year,
		&a.Item.Color, &a.Item.Tags, &a.Item.Condition, &a.Item.ImageURL, &a.Item.SourceURL, &a.Item.Description,
		&a.Item.Category, &a.Item.Artist, &a.Item.Size, &a.Item.ReadyWeeks, &a.Seller,
		&a.StartPrice, &a.MinIncrement, &a.BuyNowPrice, &a.CurrentHighestBid, &a.HighestBidderID,
		&a.Status, &a.DurationMinutes, &a.StartTime, &a.EndTime, &a.WinnerID)
	a.Sold = a.WinnerID != nil
	return a, err
}

func (s *Auctions) Get(ctx context.Context, id int64) (Auction, error) {
	a, err := scanAuction(s.pool.QueryRow(ctx, auctionSelect+" WHERE a.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Auction{}, ErrNotFound
	}
	return a, err
}

// BidEntry is one row of an auction's public bid history.
type BidEntry struct {
	BidderID  int64     `json:"-"`                      // private, see Auction.HighestBidderID
	Mine      bool      `json:"mine"`                   // placed by the viewer
	Bidder    string    `json:"bidder" example:"d***1"` // masked username
	Amount    int64     `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

// bidHistoryLimit caps the history; older bids add nothing a bidder acts on.
const bidHistoryLimit = 20

// Bids returns the latest bids on an auction, newest first. Usernames are
// masked so the history shows activity without exposing who is bidding.
func (s *Auctions) Bids(ctx context.Context, id int64) ([]BidEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.bidder_id, u.username, b.amount, b.created_at
		FROM bids b JOIN users u ON u.id = b.bidder_id
		WHERE b.auction_id = $1 ORDER BY b.amount DESC LIMIT $2`, id, bidHistoryLimit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (BidEntry, error) {
		var b BidEntry
		err := r.Scan(&b.BidderID, &b.Bidder, &b.Amount, &b.CreatedAt)
		b.Bidder = maskName(b.Bidder)
		return b, err
	})
	if err != nil || len(out) > 0 {
		return out, err
	}
	// No bids: tell an empty auction apart from one that does not exist.
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	return []BidEntry{}, nil
}

// maskName keeps the first and last letters: demo_bidder1 -> d***1.
func maskName(name string) string {
	r := []rune(name)
	if len(r) < 2 {
		return name + "***"
	}
	return string(r[0]) + "***" + string(r[len(r)-1])
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
		// Past end_time it is the closer's to mark completed. Drafts have no end_time.
		if l.ended {
			return fmt.Errorf("%w: auction has ended", ErrConflict)
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
// A bid at or above the buy-now price is a purchase at that price (buyOut).
func (s *Auctions) Bid(ctx context.Context, bidderID, id, amount int64) (Auction, error) {
	if amount <= 0 || amount > MaxMoney {
		return Auction{}, fmt.Errorf("%w: amount must be 1-%d", ErrInvalidInput, MaxMoney)
	}
	return s.mutate(ctx, id, func(tx pgx.Tx, l locked) error {
		if err := l.open(); err != nil {
			return err
		}
		if l.sellerID == bidderID {
			return fmt.Errorf("%w: sellers cannot bid on their own auction", ErrForbidden)
		}
		if l.buyNowPrice != nil && amount >= *l.buyNowPrice {
			return buyOut(ctx, tx, l, bidderID, id)
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
// auction. It stays open while the highest bid is under the buy-now price (a
// bid reaching it buys the lot itself, so a live auction is always under it).
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
		return buyOut(ctx, tx, l, buyerID, id)
	})
}

// buyOut sells the auction to buyerID at the buy-now price, inside the
// auction's lock: the current leader (who may be the buyer) gets their escrow
// back first, then the price moves from buyer to seller. The sale is recorded
// as the highest bid, so every view shows what the winner paid.
func buyOut(ctx context.Context, tx pgx.Tx, l locked, buyerID, id int64) error {
	price := *l.buyNowPrice
	ids := []int64{buyerID, l.sellerID}
	if l.highestBidderID != nil {
		ids = append(ids, *l.highestBidderID)
	}
	available, err := lockWallets(ctx, tx, ids)
	if err != nil {
		return err
	}
	if l.highestBidderID != nil {
		prev := *l.highestBid
		if err := move(ctx, tx, *l.highestBidderID, id, "release", prev, prev, -prev); err != nil {
			return err
		}
		available[*l.highestBidderID] += prev
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
	if _, err := tx.Exec(ctx,
		`UPDATE auctions SET status = 'completed', winner_id = $2, current_highest_bid = $3,
		   highest_bidder_id = $2, end_time = now()
		 WHERE id = $1`, id, buyerID, price); err != nil {
		return err
	}
	return saleNotice(ctx, tx, id, buyerID, price)
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
			if _, err := tx.Exec(ctx, "UPDATE auctions SET status = 'completed', winner_id = $2 WHERE id = $1", id, winner); err != nil {
				return err
			}
			if winner == nil {
				return nil
			}
			return saleNotice(ctx, tx, id, *winner, *bid)
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

// validItem checks and cleans seller input before it is stored and later
// rendered by clients: known brand, bounded text without NUL bytes, sane
// year, lowercase tags, and URLs limited to http(s) or our own /uploads/.
func validItem(it Item) (Item, error) {
	bad := func(msg string) (Item, error) { return Item{}, fmt.Errorf("%w: %s", ErrInvalidInput, msg) }
	it.Artist, it.Size = strings.TrimSpace(it.Artist), strings.TrimSpace(it.Size)
	switch it.Category {
	case "", "diecast":
		it.Category = "diecast"
		brand := it.Brand
		it.Brand = ""
		for _, b := range Brands {
			if strings.EqualFold(b, strings.TrimSpace(brand)) {
				it.Brand = b
			}
		}
		if it.Brand == "" {
			return bad("brand must be one of " + strings.Join(Brands, ", "))
		}
		if it.Artist != "" || it.Size != "" || it.ReadyWeeks != 0 {
			return bad("artist, size and ready_weeks are for dioramas only")
		}
	case "diorama":
		// Sold by its artist or second-hand by an owner: no diecast brand, an
		// optional artist, and new/used instead of packaging conditions.
		if strings.TrimSpace(it.Brand) != "" {
			return bad("a diorama has no diecast brand; name its artist instead")
		}
		it.Brand = ""
		if !slices.Contains(DioramaConditions, it.Condition) {
			return bad("diorama condition must be new or used")
		}
		if it.ReadyWeeks < 0 || it.ReadyWeeks > 52 {
			return bad("ready_weeks must be 0 (finished) to 52")
		}
	default:
		return bad("category must be diecast or diorama")
	}
	it.ModelName, it.Series, it.Color = strings.TrimSpace(it.ModelName), strings.TrimSpace(it.Series), strings.TrimSpace(it.Color)
	for _, f := range []struct {
		name, value string
		max         int
	}{
		{"model_name", it.ModelName, 200}, {"series", it.Series, 100}, {"color", it.Color, 100},
		{"description", it.Description, 2000}, {"image_url", it.ImageURL, 500}, {"source_url", it.SourceURL, 500},
		{"artist", it.Artist, 60}, {"size", it.Size, 50},
	} {
		if utf8.RuneCountInString(f.value) > f.max {
			return bad(fmt.Sprintf("%s must be at most %d characters", f.name, f.max))
		}
		if strings.ContainsRune(f.value, 0) {
			return bad(f.name + " contains a NUL character")
		}
	}
	if it.ModelName == "" {
		return bad("model_name is required")
	}
	if it.Year != 0 && (it.Year < 1950 || it.Year > 2100) {
		return bad("year must be 1950-2100")
	}
	tags := []string{}
	for _, t := range it.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if utf8.RuneCountInString(t) > 30 || strings.ContainsRune(t, 0) {
			return bad("each tag must be at most 30 characters")
		}
		if t != "" && !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	if len(tags) > 10 {
		return bad("at most 10 tags")
	}
	it.Tags = tags
	web := func(u string) bool { return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") }
	upload := strings.HasPrefix(it.ImageURL, "/uploads/") && !strings.Contains(it.ImageURL[len("/uploads/"):], "/")
	if it.ImageURL != "" && !web(it.ImageURL) && !upload {
		return bad("image_url must be an http(s) URL or a /uploads/ path from POST /uploads")
	}
	if it.SourceURL != "" && !web(it.SourceURL) {
		return bad("source_url must be an http(s) URL")
	}
	return it, nil
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
