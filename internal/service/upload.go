package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Upload caps over a rolling 24 hours: a count per user, and a byte budget for
// everyone, since throwaway accounts would otherwise fill the disk anyway.
const (
	UploadsPerUserPerDay       = 20
	UploadBytesPerDay    int64 = 2 << 30
)

// ReserveUpload records a photo of size bytes for userID before it is written,
// or returns ErrConflict when a daily cap is reached.
func (s *Auctions) ReserveUpload(ctx context.Context, userID int64, name string, size int64) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// ponytail: one lock serializes every reservation so two cannot both pass the
		// caps; uploads are rare. Per-user keys plus a counter row if that ever queues.
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('uploads'))"); err != nil {
			return err
		}
		var mine int
		var total int64
		err := tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE user_id = $1), coalesce(sum(size), 0)
			FROM uploads WHERE created_at > now() - interval '1 day'`, userID).Scan(&mine, &total)
		if err != nil {
			return err
		}
		if mine >= UploadsPerUserPerDay {
			return fmt.Errorf("%w: at most %d photos a day", ErrConflict, UploadsPerUserPerDay)
		}
		if total+size > UploadBytesPerDay {
			return fmt.Errorf("%w: photo uploads are full for today, try again tomorrow", ErrConflict)
		}
		_, err = tx.Exec(ctx, "INSERT INTO uploads (name, user_id, size) VALUES ($1, $2, $3)", name, userID, size)
		return err
	})
}
