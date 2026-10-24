package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/testutil"
)

func newWalletUser(t *testing.T) (*Wallets, int64) {
	t.Helper()
	pool := testutil.Pool(t)
	user, _ := registerUser(t, NewAuth(pool, testSecret))
	return NewWallets(pool), user.ID
}

func TestDepositAddsToAvailableAndWritesLedger(t *testing.T) {
	w, userID := newWalletUser(t)
	ctx := context.Background()

	got, err := w.Deposit(ctx, userID, 150_000)
	if err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if got != (Wallet{Available: 150_000}) {
		t.Fatalf("after deposit = %+v", got)
	}
	if got, _ := w.Get(ctx, userID); got != (Wallet{Available: 150_000}) {
		t.Fatalf("Get = %+v", got)
	}

	var n int
	var sum int64
	err = w.pool.QueryRow(ctx,
		"SELECT count(*), coalesce(sum(amount), 0) FROM wallet_transactions WHERE user_id = $1 AND type = 'deposit'",
		userID).Scan(&n, &sum)
	if err != nil || n != 1 || sum != 150_000 {
		t.Fatalf("ledger = %d rows, sum %d, err %v", n, sum, err)
	}
}

func TestDepositRejectsOutOfRangeAmount(t *testing.T) {
	w, userID := newWalletUser(t)
	for _, amount := range []int64{0, -1, MaxDeposit + 1} {
		if _, err := w.Deposit(context.Background(), userID, amount); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Deposit(%d) err = %v, want ErrInvalidInput", amount, err)
		}
	}
}

func TestGetUnknownWalletIsNotFound(t *testing.T) {
	w := NewWallets(testutil.Pool(t))
	if _, err := w.Get(context.Background(), -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// Concurrent deposits must never lose an update.
func TestConcurrentDepositsSumExactly(t *testing.T) {
	w, userID := newWalletUser(t)
	const n = 50
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			if _, err := w.Deposit(context.Background(), userID, 1_000); err != nil {
				t.Errorf("Deposit: %v", err)
			}
		})
	}
	wg.Wait()

	got, err := w.Get(context.Background(), userID)
	if err != nil || got.Available != n*1_000 {
		t.Fatalf("Get = %+v, %v; want available %d", got, err, n*1_000)
	}
}
