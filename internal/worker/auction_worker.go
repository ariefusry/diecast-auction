// Package worker runs background jobs. The business logic lives in service;
// a worker only decides when to call it.
package worker

import (
	"context"
	"log"
	"time"
)

// Run calls closeExpired every interval until ctx is cancelled. Errors are
// logged and retried on the next tick. Cancelling mid-pass rolls back only the
// auction in flight, which is atomic, so shutdown never leaves half a settlement.
func Run(ctx context.Context, every time.Duration, closeExpired func(context.Context) (int, error)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := closeExpired(ctx); err != nil && ctx.Err() == nil {
				log.Printf("close expired auctions: %v", err)
			} else if n > 0 {
				log.Printf("closed %d expired auctions", n)
			}
		}
	}
}
