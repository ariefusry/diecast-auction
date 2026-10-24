package db

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
)

func TestMigrateAppliesAndIsIdempotent(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; run via docker compose --profile test run --rm test")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	t.Cleanup(func() {
		conn.Exec(ctx, "DROP TABLE IF EXISTS migrate_probe, schema_migrations")
	})

	src := fstest.MapFS{
		"000001_probe.up.sql":   {Data: []byte("CREATE TABLE migrate_probe (id int);")},
		"000001_probe.down.sql": {Data: []byte("DROP TABLE migrate_probe;")},
	}

	for i := range 2 {
		if err := Migrate(url, src); err != nil {
			t.Fatalf("Migrate run %d: %v", i+1, err)
		}
	}

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass('migrate_probe') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !exists {
		t.Fatal("migrate_probe table not created")
	}
}
