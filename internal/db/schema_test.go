package db

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ariefusry/diecast-auction/migrations"
)

// seed is a minimal valid dataset: a seller, a funded bidder, one item,
// one draft auction, and one ledger row.
const seed = `
INSERT INTO users (id, email, username, password_hash) VALUES
  (1, 'seller@x.io', 'seller', 'h'), (2, 'bidder@x.io', 'bidder', 'h');
INSERT INTO wallets (user_id, available_balance) VALUES (1, 0), (2, 100000);
INSERT INTO items (id, seller_id, brand, model_name, condition)
  VALUES (1, 1, 'Mini GT', 'Nissan GT-R R35', 'mint_on_card');
INSERT INTO auctions (id, item_id, start_price, min_increment, duration)
  VALUES (1, 1, 50000, 5000, interval '1 day');
INSERT INTO wallet_transactions (id, user_id, type, amount) VALUES (1, 2, 'deposit', 100000);
`

func schemaConn(t *testing.T) *pgx.Conn {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; run via docker compose --profile test run --rm test")
	}
	if err := Migrate(url, migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

// inSeededTx runs stmt after seed inside a transaction that is always rolled back.
func inSeededTx(t *testing.T, conn *pgx.Conn, stmt string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err = tx.Exec(ctx, stmt)
	return err
}

func TestSchemaSeedIsValid(t *testing.T) {
	conn := schemaConn(t)
	if err := inSeededTx(t, conn, "SELECT 1"); err != nil {
		t.Fatalf("valid data rejected: %v", err)
	}
}

func TestSchemaRejectsInvalidData(t *testing.T) {
	conn := schemaConn(t)
	cases := map[string]string{
		"negative available balance": "UPDATE wallets SET available_balance = -1 WHERE user_id = 2",
		"negative locked balance":    "UPDATE wallets SET locked_balance = -1 WHERE user_id = 2",
		"second wallet for user":     "INSERT INTO wallets (user_id) VALUES (2)",
		"duplicate email any case":   "INSERT INTO users (email, username, password_hash) VALUES ('SELLER@x.io', 'other', 'h')",
		"unknown item condition":     "UPDATE items SET condition = 'like_new' WHERE id = 1",
		"unknown auction status":     "UPDATE auctions SET status = 'paused' WHERE id = 1",
		"non-positive start price":   "UPDATE auctions SET start_price = 0 WHERE id = 1",
		"non-positive increment":     "UPDATE auctions SET min_increment = 0 WHERE id = 1",
		"buy-now not above start":    "UPDATE auctions SET buy_now_price = 50000 WHERE id = 1",
		"highest bid without bidder": "UPDATE auctions SET current_highest_bid = 60000 WHERE id = 1",
		"non-positive bid":           "INSERT INTO bids (auction_id, bidder_id, amount) VALUES (1, 2, 0)",
		"unknown ledger type":        "INSERT INTO wallet_transactions (user_id, type, amount) VALUES (2, 'gift', 1)",
		"non-positive ledger amount": "INSERT INTO wallet_transactions (user_id, type, amount) VALUES (2, 'deposit', 0)",
		"ledger update":              "UPDATE wallet_transactions SET amount = 1 WHERE id = 1",
		"ledger delete":              "DELETE FROM wallet_transactions WHERE id = 1",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			err := inSeededTx(t, conn, stmt)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("accepted or non-postgres error (%v): %s", err, stmt)
			}
			// 23xxx = integrity constraint violation, P0001 = trigger RAISE EXCEPTION.
			if !strings.HasPrefix(pgErr.Code, "23") && pgErr.Code != "P0001" {
				t.Fatalf("rejected for the wrong reason (%s %s): %s", pgErr.Code, pgErr.Message, stmt)
			}
		})
	}
}
