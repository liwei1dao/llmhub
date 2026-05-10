package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is raised when a row is missing.
var ErrNotFound = errors.New("wallet/repo: not found")

// Account is a user's balance account.
type Account struct {
	ID                  int64
	UserID              int64
	Currency            string
	BalanceCents        int64
	FrozenCents         int64
	TotalRechargedCents int64
	TotalSpentCents     int64
	Version             int32
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// EnsureAccount returns the user's default-currency account, creating
// it on first call. Safe against concurrent creation attempts.
func (r *Repo) EnsureAccount(ctx context.Context, userID int64, currency string) (*Account, error) {
	const q = `
INSERT INTO wallet.accounts (user_id, currency)
VALUES ($1, $2)
ON CONFLICT (user_id, currency) DO UPDATE SET updated_at = wallet.accounts.updated_at
RETURNING id, user_id, currency, balance_cents, frozen_cents,
          total_recharged_cents, total_spent_cents, version, created_at, updated_at
`
	var a Account
	err := r.pool.QueryRow(ctx, q, userID, currency).Scan(
		&a.ID, &a.UserID, &a.Currency, &a.BalanceCents, &a.FrozenCents,
		&a.TotalRechargedCents, &a.TotalSpentCents, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAccount returns the user's account without creating one.
func (r *Repo) GetAccount(ctx context.Context, userID int64, currency string) (*Account, error) {
	const q = `
SELECT id, user_id, currency, balance_cents, frozen_cents,
       total_recharged_cents, total_spent_cents, version, created_at, updated_at
FROM wallet.accounts
WHERE user_id = $1 AND currency = $2
`
	var a Account
	err := r.pool.QueryRow(ctx, q, userID, currency).Scan(
		&a.ID, &a.UserID, &a.Currency, &a.BalanceCents, &a.FrozenCents,
		&a.TotalRechargedCents, &a.TotalSpentCents, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
