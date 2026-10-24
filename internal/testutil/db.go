// Package testutil holds helpers shared by integration tests. Never import it from production code.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ariefusry/diecast-auction/internal/db"
	"github.com/ariefusry/diecast-auction/migrations"
)

// Pool returns a pool on the migrated test database, or skips the test when
// TEST_DATABASE_URL is unset. Tests share one database across packages that
// run in parallel, so they must create their own rows (see Unique) instead
// of truncating tables.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; run via docker compose --profile test run --rm test")
	}
	if err := db.Migrate(url, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Unique returns a short random suffix for emails and usernames.
func Unique() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// AssertLedger fails the test unless the user's wallet equals the replay of
// their ledger. Direction rule: deposit and release credit available, lock
// moves available to locked. For buy_now and settle the auction's seller is
// credited and anyone else is debited (buy_now from available, settle from locked).
func AssertLedger(t *testing.T, pool *pgxpool.Pool, userID int64) {
	t.Helper()
	var available, locked, wantAvailable, wantLocked int64
	err := pool.QueryRow(context.Background(), `
		SELECT w.available_balance, w.locked_balance,
		       coalesce(sum(CASE
		           WHEN t.type IN ('deposit', 'release') THEN t.amount
		           WHEN t.type = 'lock' THEN -t.amount
		           WHEN t.type IN ('buy_now', 'settle') AND i.seller_id = t.user_id THEN t.amount
		           WHEN t.type = 'buy_now' THEN -t.amount
		           ELSE 0 END), 0),
		       coalesce(sum(CASE
		           WHEN t.type = 'lock' THEN t.amount
		           WHEN t.type = 'release' THEN -t.amount
		           WHEN t.type = 'settle' AND i.seller_id <> t.user_id THEN -t.amount
		           ELSE 0 END), 0)
		FROM wallets w
		LEFT JOIN wallet_transactions t ON t.user_id = w.user_id
		LEFT JOIN auctions a ON a.id = t.auction_id
		LEFT JOIN items i ON i.id = a.item_id
		WHERE w.user_id = $1
		GROUP BY w.user_id`, userID).Scan(&available, &locked, &wantAvailable, &wantLocked)
	if err != nil {
		t.Fatalf("ledger for user %d: %v", userID, err)
	}
	if available != wantAvailable || locked != wantLocked {
		t.Errorf("user %d wallet = (%d, %d), ledger replay = (%d, %d)",
			userID, available, locked, wantAvailable, wantLocked)
	}
}
