package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxDeposit caps one mock top-up (Rp10.000.000) since deposits mint money.
const MaxDeposit int64 = 10_000_000

type Wallet struct {
	Available int64 `json:"available_balance"`
	Locked    int64 `json:"locked_balance"`
}

type Wallets struct {
	pool *pgxpool.Pool
}

func NewWallets(pool *pgxpool.Pool) *Wallets {
	return &Wallets{pool: pool}
}

func (w *Wallets) Get(ctx context.Context, userID int64) (Wallet, error) {
	var wl Wallet
	err := w.pool.QueryRow(ctx,
		"SELECT available_balance, locked_balance FROM wallets WHERE user_id = $1", userID).
		Scan(&wl.Available, &wl.Locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return Wallet{}, ErrNotFound
	}
	return wl, err
}

// Deposit adds amount to the available balance and records it in the ledger,
// atomically. The single UPDATE takes the row lock, so concurrent deposits serialize.
func (w *Wallets) Deposit(ctx context.Context, userID, amount int64) (Wallet, error) {
	if amount <= 0 || amount > MaxDeposit {
		return Wallet{}, fmt.Errorf("%w: amount must be 1-%d", ErrInvalidInput, MaxDeposit)
	}
	var wl Wallet
	err := pgx.BeginFunc(ctx, w.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`UPDATE wallets SET available_balance = available_balance + $2
			 WHERE user_id = $1 RETURNING available_balance, locked_balance`,
			userID, amount).Scan(&wl.Available, &wl.Locked)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			"INSERT INTO wallet_transactions (user_id, type, amount) VALUES ($1, 'deposit', $2)",
			userID, amount)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Wallet{}, ErrNotFound
	}
	return wl, err
}
