package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Message is one line in the chat between an auction's seller and its winner.
type Message struct {
	ID        int64     `json:"id"`
	SenderID  int64     `json:"sender_id"`
	Sender    string    `json:"sender"` // username
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

// Messages returns the conversation, oldest first.
func (s *Auctions) Messages(ctx context.Context, userID, auctionID int64) ([]Message, error) {
	if err := s.participant(ctx, userID, auctionID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.sender_id, u.username, m.body, m.created_at
		FROM messages m JOIN users u ON u.id = m.sender_id
		WHERE m.auction_id = $1 ORDER BY m.id LIMIT $2`, auctionID, chatLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) {
		var m Message
		return m, r.Scan(&m.ID, &m.SenderID, &m.Sender, &m.Body, &m.CreatedAt)
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
