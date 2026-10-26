package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Message is one line in the chat between an auction's seller and its winner.
type Message struct {
	ID        int64     `json:"id"`
	SenderID  int64     `json:"-"`      // private: the seller would learn the winner's ID; 0 for System
	Mine      bool      `json:"mine"`   // sent by the viewer
	System    bool      `json:"system"` // written by the platform, see saleNotice
	Sender    string    `json:"sender"` // username, empty for System
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// chatLimit caps one conversation; arranging a shipment needs far fewer.
const chatLimit = 500

// participant checks that userID may use the auction's chat: it exists, it
// has a winner, and the user is that winner or the seller.
func (s *Auctions) participant(ctx context.Context, userID, auctionID int64) error {
	var seller int64
	var winner *int64
	err := s.pool.QueryRow(ctx,
		"SELECT i.seller_id, a.winner_id FROM auctions a JOIN items i ON i.id = a.item_id WHERE a.id = $1", auctionID).
		Scan(&seller, &winner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if winner == nil || (userID != seller && userID != *winner) {
		return fmt.Errorf("%w: only the seller and the winner can chat, after the auction is won", ErrForbidden)
	}
	return nil
}

// Messages returns the latest chatLimit messages of the conversation, oldest first.
func (s *Auctions) Messages(ctx context.Context, userID, auctionID int64) ([]Message, error) {
	if err := s.participant(ctx, userID, auctionID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT * FROM (
			SELECT m.id, coalesce(m.sender_id, 0), m.sender_id IS NULL, coalesce(u.username, ''), m.body, m.created_at
			FROM messages m LEFT JOIN users u ON u.id = m.sender_id
			WHERE m.auction_id = $1 ORDER BY m.id DESC LIMIT $2
		) latest ORDER BY id`, auctionID, chatLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) {
		var m Message
		return m, r.Scan(&m.ID, &m.SenderID, &m.System, &m.Sender, &m.Body, &m.CreatedAt)
	})
}

// Send adds a message from userID to the auction's chat.
func (s *Auctions) Send(ctx context.Context, userID, auctionID int64, body string) (Message, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 1000 || strings.ContainsRune(body, 0) || !utf8.ValidString(body) {
		return Message{}, fmt.Errorf("%w: a message is 1-1000 characters of text", ErrInvalidInput)
	}
	if err := s.participant(ctx, userID, auctionID); err != nil {
		return Message{}, err
	}
	m := Message{SenderID: userID, Body: body}
	err := s.pool.QueryRow(ctx, `
		WITH ins AS (INSERT INTO messages (auction_id, sender_id, body) VALUES ($1, $2, $3) RETURNING id, created_at)
		SELECT ins.id, ins.created_at, u.username FROM ins, users u WHERE u.id = $2`,
		auctionID, userID, body).Scan(&m.ID, &m.CreatedAt, &m.Sender)
	return m, err
}

// saleNotice opens the chat of a sold auction with a system message, inside
// the sale's transaction, so seller and winner both get an unread badge.
func saleNotice(ctx context.Context, tx pgx.Tx, auctionID, winnerID, price int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO messages (auction_id, sender_id, body)
		SELECT $1, NULL, 'Lelang selesai: terjual ke @' || username || ' seharga ' || $3 ||
		       '. Penjual dan pemenang bisa mengatur pengiriman di sini.'
		FROM users WHERE id = $2`, auctionID, winnerID, rupiah(price))
	return err
}

// rupiah formats an amount the way the site shows it: Rp1.250.000.
func rupiah(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return "Rp" + s
}

// Conversation is one chat in a user's inbox, as that user sees it.
type Conversation struct {
	AuctionID int64      `json:"auction_id"`
	Title     string     `json:"title"`
	ImageURL  string     `json:"image_url"`
	Price     int64      `json:"price"`   // what the winner paid
	Other     string     `json:"other"`   // the other side's username
	Selling   bool       `json:"selling"` // the viewer is the seller
	LastBody  string     `json:"last_body"`
	LastMine  bool       `json:"last_mine"` // the last message is the viewer's
	LastAt    *time.Time `json:"last_at"`   // nil until someone writes
	Unread    int        `json:"unread"`    // messages from the other side or the system after the read marker
}

// inboxLimit caps the inbox; older chats stay reachable from the auction page.
const inboxLimit = 100

// Conversations lists every won auction userID sold or won, latest activity first.
func (s *Auctions) Conversations(ctx context.Context, userID int64) ([]Conversation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, i.model_name, coalesce(i.image_url, ''), coalesce(a.current_highest_bid, a.buy_now_price, 0),
		       CASE WHEN i.seller_id = $1 THEN w.username ELSE su.username END, i.seller_id = $1,
		       coalesce(lm.body, ''), coalesce(lm.sender_id = $1, false), lm.created_at,
		       (SELECT count(*) FROM messages m
		        WHERE m.auction_id = a.id AND m.sender_id IS DISTINCT FROM $1 AND m.id > coalesce(r.last_read_message_id, 0))
		FROM auctions a
		JOIN items i ON i.id = a.item_id
		JOIN users su ON su.id = i.seller_id
		JOIN users w ON w.id = a.winner_id
		LEFT JOIN LATERAL (SELECT body, sender_id, created_at FROM messages
		                   WHERE auction_id = a.id ORDER BY id DESC LIMIT 1) lm ON true
		LEFT JOIN chat_reads r ON r.auction_id = a.id AND r.user_id = $1
		WHERE i.seller_id = $1 OR a.winner_id = $1
		ORDER BY coalesce(lm.created_at, a.end_time) DESC, a.id DESC
		LIMIT $2`, userID, inboxLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Conversation, error) {
		var c Conversation
		return c, r.Scan(&c.AuctionID, &c.Title, &c.ImageURL, &c.Price, &c.Other, &c.Selling,
			&c.LastBody, &c.LastMine, &c.LastAt, &c.Unread)
	})
}

// MarkRead records that userID has read the auction's chat up to message upTo.
// The marker only moves forward, so a late request cannot unread anything.
func (s *Auctions) MarkRead(ctx context.Context, userID, auctionID, upTo int64) error {
	if err := s.participant(ctx, userID, auctionID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_reads (auction_id, user_id, last_read_message_id) VALUES ($1, $2, $3)
		ON CONFLICT (auction_id, user_id)
		DO UPDATE SET last_read_message_id = GREATEST(chat_reads.last_read_message_id, EXCLUDED.last_read_message_id)`,
		auctionID, userID, upTo)
	return err
}
