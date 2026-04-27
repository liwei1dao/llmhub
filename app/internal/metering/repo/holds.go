package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ExpiredHold is the minimum projection a reaper needs.
type ExpiredHold struct {
	RequestID   string
	UserID      int64
	AccountID   int64
	AmountCents int64
}

// ListExpiredHolds returns holds that should be released (status='held'
// AND expires_at < NOW()), bounded by limit.
func (r *Repo) ListExpiredHolds(ctx context.Context, limit int) ([]ExpiredHold, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const sql = `
SELECT request_id, user_id, account_id, amount_cents
FROM metering.balance_holds
WHERE status = 'held' AND expires_at < NOW()
ORDER BY expires_at ASC
LIMIT $1
`
	rows, err := r.pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpiredHold
	for rows.Next() {
		var h ExpiredHold
		if err := rows.Scan(&h.RequestID, &h.UserID, &h.AccountID, &h.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// MarkHoldExpired flips a single hold's status to 'released' so the
// reaper can stay idempotent: subsequent passes won't see it again.
func (r *Repo) MarkHoldExpired(ctx context.Context, requestID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE metering.balance_holds SET status = 'released' WHERE request_id = $1 AND status = 'held'`,
		requestID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
