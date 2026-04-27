package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Recharge is a single recharge order.
type Recharge struct {
	ID             int64
	OrderNo        string
	UserID         int64
	AmountCents    int64
	Channel        string // alipay / wechat / stripe / manual
	ChannelOrderID *string
	Status         string // pending / paid / failed / refunded / cancelled
	PaidAt         *time.Time
	CreatedAt      time.Time
}

// CreateRecharge inserts a pending recharge order; channel-side
// payment is initiated separately by the caller (PSP integration).
func (r *Repo) CreateRecharge(ctx context.Context, userID int64, orderNo, channel string, amountCents int64) (*Recharge, error) {
	if amountCents <= 0 {
		return nil, errors.New("wallet/repo: non-positive amount")
	}
	const sql = `
INSERT INTO wallet.recharges (order_no, user_id, amount_cents, channel)
VALUES ($1, $2, $3, $4)
RETURNING id, status, created_at
`
	rg := &Recharge{
		OrderNo:     orderNo,
		UserID:      userID,
		AmountCents: amountCents,
		Channel:     channel,
	}
	if err := r.pool.QueryRow(ctx, sql, orderNo, userID, amountCents, channel).Scan(
		&rg.ID, &rg.Status, &rg.CreatedAt,
	); err != nil {
		return nil, err
	}
	return rg, nil
}

// GetRechargeByOrderNo returns a recharge by its order id.
func (r *Repo) GetRechargeByOrderNo(ctx context.Context, orderNo string) (*Recharge, error) {
	const sql = `
SELECT id, order_no, user_id, amount_cents, channel, channel_order_id, status, paid_at, created_at
FROM wallet.recharges WHERE order_no = $1
`
	var rg Recharge
	err := r.pool.QueryRow(ctx, sql, orderNo).Scan(
		&rg.ID, &rg.OrderNo, &rg.UserID, &rg.AmountCents, &rg.Channel, &rg.ChannelOrderID,
		&rg.Status, &rg.PaidAt, &rg.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &rg, err
}

// ConfirmRecharge atomically marks the order paid AND credits the
// user's wallet. Idempotent: if the order is already 'paid' the
// caller still gets a nil error and the wallet isn't double-credited.
func (r *Repo) ConfirmRecharge(ctx context.Context, orderNo string, channelOrderID string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// Lock the recharge row first so two concurrent webhook callbacks
		// can't both flip it to paid.
		var (
			rid         int64
			userID      int64
			amountCents int64
			status      string
		)
		const sel = `
SELECT id, user_id, amount_cents, status
FROM wallet.recharges
WHERE order_no = $1
FOR UPDATE
`
		if err := tx.QueryRow(ctx, sel, orderNo).Scan(&rid, &userID, &amountCents, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == "paid" {
			return nil // idempotent
		}
		if status != "pending" {
			return errors.New("wallet/repo: recharge not pending")
		}

		const upd = `
UPDATE wallet.recharges
SET status = 'paid', channel_order_id = NULLIF($2, ''), paid_at = NOW()
WHERE id = $1
`
		if _, err := tx.Exec(ctx, upd, rid, channelOrderID); err != nil {
			return err
		}

		// Credit balance + add transaction row in the same tx so the
		// audit trail is exact.
		var accountID, balanceAfter int64
		const credit = `
UPDATE wallet.accounts
SET balance_cents          = balance_cents + $1,
    total_recharged_cents  = total_recharged_cents + $1,
    version                = version + 1,
    updated_at             = NOW()
WHERE user_id = $2 AND currency = 'CNY'
RETURNING id, balance_cents
`
		if err := tx.QueryRow(ctx, credit, amountCents, userID).Scan(&accountID, &balanceAfter); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("wallet/repo: wallet account missing")
			}
			return err
		}

		const txIns = `
INSERT INTO wallet.transactions (account_id, type, amount_cents, balance_after, related_id, related_type, memo)
VALUES ($1, 'recharge', $2, $3, $4, 'recharge', 'recharge confirmed')
`
		_, err := tx.Exec(ctx, txIns, accountID, amountCents, balanceAfter, orderNo)
		return err
	})
}

// ListRechargesByUser returns the user's recent recharge orders.
func (r *Repo) ListRechargesByUser(ctx context.Context, userID int64, limit int) ([]Recharge, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const sql = `
SELECT id, order_no, user_id, amount_cents, channel, channel_order_id, status, paid_at, created_at
FROM wallet.recharges
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, sql, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Recharge
	for rows.Next() {
		var rg Recharge
		if err := rows.Scan(
			&rg.ID, &rg.OrderNo, &rg.UserID, &rg.AmountCents, &rg.Channel, &rg.ChannelOrderID,
			&rg.Status, &rg.PaidAt, &rg.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rg)
	}
	return out, rows.Err()
}
