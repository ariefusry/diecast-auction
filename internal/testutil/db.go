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
