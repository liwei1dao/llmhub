// Package repo is the data-access layer for metering: call logs and
// reconciliation snapshots.
package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo exposes metering queries.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ReconRow is one row from metering.reconciliation.
type ReconRow struct {
	ID                  int64
	Day                 time.Time
	ProviderID          string
	PoolAccountID       int64
	PlatformCostCents   float64
	UpstreamBillCents   *float64
	DiffCents           *float64
	DiffRatio           *float64
	Status              string
	Notes               *string
	CreatedAt           time.Time
}

// ListRecon returns reconciliation rows filtered by day/provider, newest first.
func (r *Repo) ListRecon(ctx context.Context, day time.Time, providerID string, limit int) ([]ReconRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const sql = `
SELECT id, day, provider_id, pool_account_id,
       platform_cost_cents, upstream_bill_cents, diff_cents, diff_ratio,
       status, notes, created_at
FROM metering.reconciliation
WHERE ($1 = TRUE OR day = $2::date)
  AND ($3 = '' OR provider_id = $3)
ORDER BY day DESC, id DESC
LIMIT $4
`
	allDays := day.IsZero()
	rows, err := r.pool.Query(ctx, sql, allDays, day, providerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReconRow
	for rows.Next() {
		var x ReconRow
		if err := rows.Scan(
			&x.ID, &x.Day, &x.ProviderID, &x.PoolAccountID,
			&x.PlatformCostCents, &x.UpstreamBillCents, &x.DiffCents, &x.DiffRatio,
			&x.Status, &x.Notes, &x.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
