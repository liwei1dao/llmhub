package repo

import (
	"context"
	"time"
)

// AdminUser is the verbose admin-view of an IAM user.
type AdminUser struct {
	ID                   int64
	Email                *string
	Phone                *string
	Status               string
	RiskScore            int16
	QPSLimit             int32
	DailySpendLimitCents int64
	CreatedAt            time.Time
	LastLoginAt          *time.Time
}

// ListAdminUsers returns a cursor-less first page of users matching the filter.
// Intended for operator dashboards, not for end-user traffic.
func (r *Repo) ListAdminUsers(ctx context.Context, q string, limit int) ([]AdminUser, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const sql = `
SELECT id, email, phone, status, risk_score, qps_limit,
       daily_spend_limit_cents, created_at, last_login_at
FROM iam.users
WHERE ($1 = '' OR email ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%')
ORDER BY created_at DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, sql, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Phone, &u.Status, &u.RiskScore, &u.QPSLimit,
			&u.DailySpendLimitCents, &u.CreatedAt, &u.LastLoginAt,
		); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
