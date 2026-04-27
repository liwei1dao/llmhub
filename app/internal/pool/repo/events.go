package repo

import (
	"context"
	"time"
)

// AccountEvent is one row from pool.account_events.
type AccountEvent struct {
	ID        int64
	AccountID int64
	EventType string
	FromState *string
	ToState   *string
	Reason    *string
	CreatedAt time.Time
}

// ListAccountEvents returns the event log for an account, newest first.
func (r *Repo) ListAccountEvents(ctx context.Context, accountID int64, limit int) ([]AccountEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const sql = `
SELECT id, account_id, event_type, from_state, to_state, reason, created_at
FROM pool.account_events
WHERE account_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, sql, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountEvent
	for rows.Next() {
		var e AccountEvent
		if err := rows.Scan(&e.ID, &e.AccountID, &e.EventType, &e.FromState, &e.ToState, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
