package repo

import (
	"context"
	"time"
)

// RunDailyRecon aggregates call_logs for the given day into the
// metering.reconciliation table. One row per (provider, pool_account)
// is upserted so the cron can safely re-run on partial failures.
//
// `upstream_bill_cents` is left NULL: the platform-side cost is
// authoritative until the M11 sync job pulls real upstream bills and
// fills the diff_cents column.
func (r *Repo) RunDailyRecon(ctx context.Context, day time.Time) (int64, error) {
	const sql = `
INSERT INTO metering.reconciliation (
    day, provider_id, pool_account_id, platform_cost_cents, status
)
SELECT
    $1::date AS day,
    provider_id,
    pool_account_id,
    SUM(cost_retail_cents) AS platform_cost_cents,
    'ok' AS status
FROM metering.call_logs
WHERE ts >= $1::date AND ts < ($1::date + INTERVAL '1 day')
  AND status = 'success'
GROUP BY provider_id, pool_account_id
ON CONFLICT (day, pool_account_id) DO UPDATE SET
    platform_cost_cents = EXCLUDED.platform_cost_cents,
    status              = EXCLUDED.status
`
	tag, err := r.pool.Exec(ctx, sql, day.UTC().Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
