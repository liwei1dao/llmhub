package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Subscription is one row in iam.subscriptions — a user's active or
// historical subscription to a SKU. Quota counters are integers; the
// unit semantics depend on the SKU.billing_unit (1k_tokens / minute /
// image / page / query). The /sdk/usage/report path increments
// quota_used and daily_used in the same units.
type Subscription struct {
	ID              int64
	UserID          int64
	SKUID           string
	PlanKind        string // monthly / prepaid / trial
	PlanName        string
	QuotaTotal      int64
	QuotaUsed       int64
	PeriodStart     time.Time
	PeriodEnd       time.Time
	AutoRenew       bool
	Status          string
	QPSLimit        int32
	DailyQuotaLimit *int64
	DailyUsed       int64
	DailyUsedDate   time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SubscriptionRepo is the data-access surface for iam.subscriptions.
type SubscriptionRepo struct{ r *Repo }

// Subscriptions is sugar.
func (r *Repo) Subscriptions() *SubscriptionRepo { return &SubscriptionRepo{r: r} }

// Create inserts a subscription row.
func (sr *SubscriptionRepo) Create(ctx context.Context, s *Subscription) (int64, error) {
	const sql = `
INSERT INTO iam.subscriptions
       (user_id, sku_id, plan_kind, plan_name,
        quota_total, period_start, period_end, auto_renew,
        qps_limit, daily_quota_limit)
VALUES ($1, $2, $3, NULLIF($4,''),
        $5, COALESCE($6, NOW()), $7, $8,
        COALESCE(NULLIF($9,0), 10), $10)
RETURNING id, status, created_at, updated_at
`
	err := sr.r.pool.QueryRow(ctx, sql,
		s.UserID, s.SKUID, s.PlanKind, s.PlanName,
		s.QuotaTotal, s.PeriodStart, s.PeriodEnd, s.AutoRenew,
		s.QPSLimit, s.DailyQuotaLimit,
	).Scan(&s.ID, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	return s.ID, err
}

// GetActiveByUserSKU resolves the active subscription a user holds for
// a given SKU. Returns ErrNotFound if no active subscription exists.
// This is the /sdk/credentials/issue gating query.
func (sr *SubscriptionRepo) GetActiveByUserSKU(ctx context.Context, userID int64, skuID string) (*Subscription, error) {
	const sql = `
SELECT id, user_id, sku_id, plan_kind, COALESCE(plan_name,''),
       quota_total, quota_used,
       period_start, period_end, auto_renew, status,
       qps_limit, daily_quota_limit, daily_used, daily_used_date,
       created_at, updated_at
FROM iam.subscriptions
WHERE user_id = $1 AND sku_id = $2
  AND status = 'active'
  AND period_end > NOW()
LIMIT 1
`
	var s Subscription
	err := sr.r.pool.QueryRow(ctx, sql, userID, skuID).Scan(
		&s.ID, &s.UserID, &s.SKUID, &s.PlanKind, &s.PlanName,
		&s.QuotaTotal, &s.QuotaUsed,
		&s.PeriodStart, &s.PeriodEnd, &s.AutoRenew, &s.Status,
		&s.QPSLimit, &s.DailyQuotaLimit, &s.DailyUsed, &s.DailyUsedDate,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

// ListByUser returns every active subscription belonging to a user.
// Used by the SDK's `/sdk/services` discovery endpoint and the user
// console subscription view.
func (sr *SubscriptionRepo) ListByUser(ctx context.Context, userID int64) ([]Subscription, error) {
	const sql = `
SELECT id, user_id, sku_id, plan_kind, COALESCE(plan_name,''),
       quota_total, quota_used,
       period_start, period_end, auto_renew, status,
       qps_limit, daily_quota_limit, daily_used, daily_used_date,
       created_at, updated_at
FROM iam.subscriptions
WHERE user_id = $1 AND status = 'active'
ORDER BY created_at DESC
`
	rows, err := sr.r.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.SKUID, &s.PlanKind, &s.PlanName,
			&s.QuotaTotal, &s.QuotaUsed,
			&s.PeriodStart, &s.PeriodEnd, &s.AutoRenew, &s.Status,
			&s.QPSLimit, &s.DailyQuotaLimit, &s.DailyUsed, &s.DailyUsedDate,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get fetches a subscription by id (any status). Used by admin patch /
// cancel paths where the caller already has the row id.
func (sr *SubscriptionRepo) Get(ctx context.Context, id int64) (*Subscription, error) {
	const sql = `
SELECT id, user_id, sku_id, plan_kind, COALESCE(plan_name,''),
       quota_total, quota_used,
       period_start, period_end, auto_renew, status,
       qps_limit, daily_quota_limit, daily_used, daily_used_date,
       created_at, updated_at
FROM iam.subscriptions
WHERE id = $1
`
	var s Subscription
	err := sr.r.pool.QueryRow(ctx, sql, id).Scan(
		&s.ID, &s.UserID, &s.SKUID, &s.PlanKind, &s.PlanName,
		&s.QuotaTotal, &s.QuotaUsed,
		&s.PeriodStart, &s.PeriodEnd, &s.AutoRenew, &s.Status,
		&s.QPSLimit, &s.DailyQuotaLimit, &s.DailyUsed, &s.DailyUsedDate,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

// SubscriptionPatch carries the optional fields an admin can change on
// an existing subscription. Nil pointers are left untouched. Building
// the SET clause dynamically keeps the SQL straightforward and lets us
// add fields without touching every callsite.
type SubscriptionPatch struct {
	QuotaTotal      *int64
	PeriodEnd       *time.Time
	AutoRenew       *bool
	QPSLimit        *int32
	DailyQuotaLimit *int64
	Status          *string // active / suspended / cancelled / expired
	PlanName        *string
}

// Patch applies a partial update. Returns ErrNotFound if no row matches.
func (sr *SubscriptionRepo) Patch(ctx context.Context, id int64, p SubscriptionPatch) error {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if p.QuotaTotal != nil {
		add("quota_total", *p.QuotaTotal)
	}
	if p.PeriodEnd != nil {
		add("period_end", *p.PeriodEnd)
	}
	if p.AutoRenew != nil {
		add("auto_renew", *p.AutoRenew)
	}
	if p.QPSLimit != nil {
		add("qps_limit", *p.QPSLimit)
	}
	if p.DailyQuotaLimit != nil {
		add("daily_quota_limit", *p.DailyQuotaLimit)
	}
	if p.Status != nil {
		add("status", *p.Status)
	}
	if p.PlanName != nil {
		add("plan_name", *p.PlanName)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	q := "UPDATE iam.subscriptions SET " + strings.Join(sets, ", ") +
		", updated_at = NOW() WHERE id = $" + itoa(len(args))
	tag, err := sr.r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Cancel flips the subscription to status='cancelled'. Different from
// Patch+status only in that we also stamp updated_at and don't allow
// reactivating from this method (admin uses Patch for that path).
func (sr *SubscriptionRepo) Cancel(ctx context.Context, id int64) error {
	const sql = `
UPDATE iam.subscriptions
SET status = 'cancelled', auto_renew = FALSE, updated_at = NOW()
WHERE id = $1 AND status = 'active'
`
	tag, err := sr.r.pool.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// itoa converts a positive int to its base-10 string. Inlined to avoid
// pulling strconv just for $N placeholders.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// AddUsage atomically increments quota_used and daily_used. Resets
// daily_used to the new units when the day rolls over. Returns the
// post-update remaining quota so the caller can decide whether to
// suspend the subscription.
//
// Returns (remainingTotal, remainingDaily, err). For NULL daily limit,
// remainingDaily = -1 (sentinel: unlimited).
func (sr *SubscriptionRepo) AddUsage(ctx context.Context, userID int64, skuID string, units int64) (int64, int64, error) {
	const sql = `
UPDATE iam.subscriptions
SET quota_used = quota_used + $3,
    daily_used = CASE WHEN daily_used_date = CURRENT_DATE
                      THEN daily_used + $3
                      ELSE $3 END,
    daily_used_date = CURRENT_DATE,
    updated_at = NOW()
WHERE user_id = $1 AND sku_id = $2 AND status = 'active'
RETURNING (quota_total - quota_used) AS remaining_total,
          CASE WHEN daily_quota_limit IS NULL THEN -1
               ELSE daily_quota_limit - daily_used END AS remaining_daily
`
	var rt, rd int64
	if err := sr.r.pool.QueryRow(ctx, sql, userID, skuID, units).Scan(&rt, &rd); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}
	return rt, rd, nil
}
