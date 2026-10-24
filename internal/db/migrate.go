// Package db holds database plumbing shared by the API, seeder, and tests.
package db

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies every pending up migration in src. Already-applied
// migrations are a no-op, so it is safe to call on every API start.
func Migrate(databaseURL string, src fs.FS) error {
	source, err := iofs.New(src, ".")
	if err != nil {
		return fmt.Errorf("migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, toPgx5Scheme(databaseURL))
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func toPgx5Scheme(url string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(url, prefix) {
			return "pgx5://" + strings.TrimPrefix(url, prefix)
		}
	}
	return url
}
