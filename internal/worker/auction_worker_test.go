package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunTicksUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		Run(ctx, time.Millisecond, func(context.Context) (int, error) {
			// Errors are logged, not fatal: the next tick retries.
			if calls.Add(1) == 1 {
				return 0, errors.New("transient")
			}
			return 1, nil
		})
		close(done)
	}()

	for calls.Load() < 3 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
