package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Hold is a pre-authorized balance freeze created before an AI call.
type Hold struct {
	RequestID   string
	UserID      int64
	AccountID   int64
	AmountCents int64
	Status      string // held / released / settled
	ExpiresAt   time.Time
	CreatedAt   time.Time
	SettledAt   *time.Time
}

// Freeze atomically moves amount cents from balance → frozen and
// inserts a matching wallet.transactions row + metering.balance_holds row.
//
// Returns ErrInsufficient if balance cannot cover the amount.
func (r *Repo) Freeze(ctx context.Context, requestID string, userID, accountID, amountCents int64, ttl time.Duration) error {
	if amountCents <= 0 {
		return errors.New("wallet/repo: non-positive amount")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		const upd = `
UPDATE wallet.accounts
SET frozen_cents = frozen_cents + $1,
    version = version + 1,
    updated_at = NOW()
WHERE id = $2
  AND balance_cents - frozen_cents >= $1
RETURNING balance_cents, frozen_cents
`
		var balAfter, frozenAfter int64
		err := tx.QueryRow(ctx, upd, amountCents, accountID).Scan(&balAfter, &frozenAfter)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficient
		}
		if err != nil {
			return err
		}

		const txIns = `
INSERT INTO wallet.transactions (account_id, type, amount_cents, balance_after, related_id, related_type)
VALUES ($1, 'freeze', $2, $3, $4, 'call')
`
		if _, err := tx.Exec(ctx, txIns, accountID, -amountCents, balAfter, requestID); err != nil {
			return err
		}

		const holdIns = `
INSERT INTO metering.balance_holds (request_id, user_id, account_id, amount_cents, status, expires_at)
VALUES ($1, $2, $3, $4, 'held', NOW() + $5::interval)
`
		if _, err := tx.Exec(ctx, holdIns, requestID, userID, accountID, amountCents, ttl.String()); err != nil {
			return err
		}
		return nil
	})
}

// ErrInsufficient signals that a freeze could not be satisfied.
var ErrInsufficient = errors.New("wallet/repo: insufficient funds")

// Settle marks the hold settled and applies the actual cost: unfreezes
// the held amount and debits the real cost from balance.
// If actualCents > held, the excess is still debited from remaining balance.
// If actualCents < held, the difference is returned to the user.
func (r *Repo) Settle(ctx context.Context, requestID string, actualCents int64) error {
	if actualCents < 0 {
		return errors.New("wallet/repo: negative actual cost")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		h, err := holdForUpdate(ctx, tx, requestID)
		if err != nil {
			return err
		}
		if h.Status != "held" {
			return errors.New("wallet/repo: hold already closed")
		}

		const q = `
UPDATE wallet.accounts
SET balance_cents = balance_cents - $1,
    frozen_cents  = frozen_cents  - $2,
    total_spent_cents = total_spent_cents + $1,
    version = version + 1,
    updated_at = NOW()
WHERE id = $3
RETURNING balance_cents
`
		var balAfter int64
		if err := tx.QueryRow(ctx, q, actualCents, h.AmountCents, h.AccountID).Scan(&balAfter); err != nil {
			return err
		}

		const txs = `
INSERT INTO wallet.transactions (account_id, type, amount_cents, balance_after, related_id, related_type)
VALUES
  ($1, 'unfreeze', $2, $3, $4, 'call'),
  ($1, 'spend',    $5, $3, $4, 'call')
`
		if _, err := tx.Exec(ctx, txs, h.AccountID, h.AmountCents, balAfter, requestID, -actualCents); err != nil {
			return err
		}

		const closeHold = `
UPDATE metering.balance_holds
SET status = 'settled', settled_at = NOW()
WHERE request_id = $1
`
		_, err = tx.Exec(ctx, closeHold, requestID)
		return err
	})
}

// Release returns the full held amount to the balance (used on call failure).
func (r *Repo) Release(ctx context.Context, requestID string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		h, err := holdForUpdate(ctx, tx, requestID)
		if err != nil {
			return err
		}
		if h.Status != "held" {
			return errors.New("wallet/repo: hold already closed")
		}

		const q = `
UPDATE wallet.accounts
SET frozen_cents = frozen_cents - $1,
    version = version + 1,
    updated_at = NOW()
WHERE id = $2
RETURNING balance_cents
`
		var balAfter int64
		if err := tx.QueryRow(ctx, q, h.AmountCents, h.AccountID).Scan(&balAfter); err != nil {
			return err
		}

		const txs = `
INSERT INTO wallet.transactions (account_id, type, amount_cents, balance_after, related_id, related_type)
VALUES ($1, 'unfreeze', $2, $3, $4, 'call')
`
		if _, err := tx.Exec(ctx, txs, h.AccountID, h.AmountCents, balAfter, requestID); err != nil {
			return err
		}

		const closeHold = `
UPDATE metering.balance_holds SET status = 'released' WHERE request_id = $1
`
		_, err = tx.Exec(ctx, closeHold, requestID)
		return err
	})
}

// holdForUpdate locks the hold row for the duration of the transaction.
func holdForUpdate(ctx context.Context, tx pgx.Tx, requestID string) (*Hold, error) {
	const q = `
SELECT request_id, user_id, account_id, amount_cents, status, expires_at, created_at, settled_at
FROM metering.balance_holds
WHERE request_id = $1
FOR UPDATE
`
	var h Hold
	err := tx.QueryRow(ctx, q, requestID).Scan(
		&h.RequestID, &h.UserID, &h.AccountID, &h.AmountCents,
		&h.Status, &h.ExpiresAt, &h.CreatedAt, &h.SettledAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrHoldNotFound
	}
	return &h, err
}

// ErrHoldNotFound matches the service-layer sentinel.
var ErrHoldNotFound = errors.New("wallet/repo: hold not found")
