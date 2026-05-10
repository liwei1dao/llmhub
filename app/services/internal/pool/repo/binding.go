package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ServiceBinding mirrors a row in pool.credential_services. It is the
// smallest schedulable unit — one row per (credential, capability)
// pair. The scheduler hot-path picks bindings, not credentials.
type ServiceBinding struct {
	ID                  int64
	CredentialID        int64
	Capability          string
	Tier                string
	QPSLimit            *int32
	DailyLimitCents     *int64
	QuotaTotalCents     *int64
	DailyUsedCents      int64
	QuotaUsedCents      int64
	QuotaResetAt        *time.Time
	CostBasisCents      float64
	HealthScore         int16
	ConsecutiveFailures int32
	LastSuccessAt       *time.Time
	LastErrorAt         *time.Time
	LastErrorCode       string
	Status              string
	CooldownUntil       *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// BindingRepo is the data-access surface for pool.credential_services.
type BindingRepo struct{ r *Repo }

// NewBindingRepo binds to the shared pool.
func NewBindingRepo(r *Repo) *BindingRepo { return &BindingRepo{r: r} }

// Bindings is sugar for repo.NewBindingRepo.
func (r *Repo) Bindings() *BindingRepo { return NewBindingRepo(r) }

// Create inserts a single binding row. Caller must validate that
// b.Capability is in the allowed-set of the credential's product
// (catalog.Products[product_id].AllowedCapabilities).
func (br *BindingRepo) Create(ctx context.Context, b *ServiceBinding) (int64, error) {
	return br.createTx(ctx, br.r.pool, b)
}

func (br *BindingRepo) createTx(ctx context.Context, q querier, b *ServiceBinding) (int64, error) {
	const sql = `
INSERT INTO pool.credential_services
       (credential_id, capability, tier, qps_limit, daily_limit_cents,
        quota_total_cents, cost_basis_cents, status)
VALUES ($1, $2, COALESCE(NULLIF($3,''),'pro'), $4, $5,
        $6, $7, COALESCE(NULLIF($8,''),'active'))
RETURNING id, tier, status, health_score, created_at, updated_at
`
	var id int64
	err := q.QueryRow(ctx, sql,
		b.CredentialID, b.Capability, b.Tier,
		b.QPSLimit, b.DailyLimitCents,
		b.QuotaTotalCents, b.CostBasisCents, b.Status,
	).Scan(&id, &b.Tier, &b.Status, &b.HealthScore, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return 0, err
	}
	b.ID = id
	return id, nil
}

// Get fetches a single binding by id.
func (br *BindingRepo) Get(ctx context.Context, id int64) (*ServiceBinding, error) {
	const q = `
SELECT id, credential_id, capability, tier,
       qps_limit, daily_limit_cents, quota_total_cents,
       daily_used_cents, quota_used_cents, quota_reset_at,
       cost_basis_cents, health_score, consecutive_failures,
       last_success_at, last_error_at, COALESCE(last_error_code,''),
       status, cooldown_until,
       created_at, updated_at
FROM pool.credential_services
WHERE id = $1
`
	b := &ServiceBinding{}
	if err := br.r.pool.QueryRow(ctx, q, id).Scan(
		&b.ID, &b.CredentialID, &b.Capability, &b.Tier,
		&b.QPSLimit, &b.DailyLimitCents, &b.QuotaTotalCents,
		&b.DailyUsedCents, &b.QuotaUsedCents, &b.QuotaResetAt,
		&b.CostBasisCents, &b.HealthScore, &b.ConsecutiveFailures,
		&b.LastSuccessAt, &b.LastErrorAt, &b.LastErrorCode,
		&b.Status, &b.CooldownUntil,
		&b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return b, nil
}

// ListByCredential returns every binding belonging to a credential.
// Order: capability ASC, for stable admin display.
func (br *BindingRepo) ListByCredential(ctx context.Context, credID int64) ([]ServiceBinding, error) {
	const q = `
SELECT id, credential_id, capability, tier,
       qps_limit, daily_limit_cents, quota_total_cents,
       daily_used_cents, quota_used_cents, quota_reset_at,
       cost_basis_cents, health_score, consecutive_failures,
       last_success_at, last_error_at, COALESCE(last_error_code,''),
       status, cooldown_until,
       created_at, updated_at
FROM pool.credential_services
WHERE credential_id = $1
ORDER BY capability ASC
`
	rows, err := br.r.pool.Query(ctx, q, credID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBindings(rows)
}

// PickQuery narrows the scheduler hot-path lookup.
type PickQuery struct {
	ProductID  string  // catalog.Products id, e.g. "volc.ark"
	Capability string  // catalog.Capabilities id, e.g. "chat"
	MinHealth  int16   // bindings below this are excluded
	Limit      int     // max candidates to return; defaults to 50
}

// Pick returns active+healthy bindings for (product, capability) along
// with their owning credential ids and vault refs, in scheduler-preferred
// order (health DESC, cost ASC, daily_used ASC).
//
// The returned slice is the candidate set; the scheduler applies its
// own scoring/sticky logic on top.
func (br *BindingRepo) Pick(ctx context.Context, q PickQuery) ([]PickedBinding, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	const sql = `
SELECT b.id, b.credential_id, b.capability, b.tier,
       b.qps_limit, b.daily_limit_cents, b.daily_used_cents,
       b.cost_basis_cents, b.health_score, b.status,
       c.vendor_id, c.product_id, c.account_id, c.auth_payload_ref,
       c.health_score, c.status, c.isolation_group_id
FROM pool.credential_services b
JOIN pool.credentials      c ON c.id  = b.credential_id
JOIN pool.vendor_accounts  v ON v.id  = c.account_id
WHERE b.status = 'active'
  AND c.status = 'active'
  AND v.status = 'active'
  AND c.product_id = $1
  AND b.capability = $2
  AND b.health_score >= $3
  AND (b.daily_limit_cents IS NULL OR b.daily_used_cents < b.daily_limit_cents)
ORDER BY b.health_score DESC, b.cost_basis_cents ASC, b.daily_used_cents ASC
LIMIT $4
`
	rows, err := br.r.pool.Query(ctx, sql,
		q.ProductID, q.Capability, q.MinHealth, q.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PickedBinding
	for rows.Next() {
		var p PickedBinding
		if err := rows.Scan(
			&p.BindingID, &p.CredentialID, &p.Capability, &p.Tier,
			&p.QPSLimit, &p.DailyLimitCents, &p.DailyUsedCents,
			&p.CostBasisCents, &p.BindingHealth, &p.BindingStatus,
			&p.VendorID, &p.ProductID, &p.AccountID, &p.AuthPayloadRef,
			&p.CredentialHealth, &p.CredentialStatus, &p.IsolationGroupID,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PickedBinding is the joined view returned by Pick — enough for the
// scheduler to choose, and for the gateway to fetch credentials from
// vault and call the upstream adapter without a second DB round-trip.
type PickedBinding struct {
	BindingID         int64
	CredentialID      int64
	Capability        string
	Tier              string
	QPSLimit          *int32
	DailyLimitCents   *int64
	DailyUsedCents    int64
	CostBasisCents    float64
	BindingHealth     int16
	BindingStatus     string
	VendorID          string
	ProductID         string
	AccountID         int64
	AuthPayloadRef    string
	CredentialHealth  int16
	CredentialStatus  string
	IsolationGroupID  *int64
}

// AdjustHealth nudges binding health, clamped to [0,100]. Returns new score.
func (br *BindingRepo) AdjustHealth(ctx context.Context, id int64, delta int, errCode string) (int16, error) {
	const q = `
UPDATE pool.credential_services
SET health_score = GREATEST(0, LEAST(100, health_score + $1)),
    consecutive_failures = CASE WHEN $1 < 0 THEN consecutive_failures + 1 ELSE 0 END,
    last_error_at  = CASE WHEN $1 < 0 THEN NOW() ELSE last_error_at END,
    last_error_code = CASE WHEN $1 < 0 THEN NULLIF($2,'') ELSE last_error_code END,
    last_success_at = CASE WHEN $1 > 0 THEN NOW() ELSE last_success_at END,
    updated_at = NOW()
WHERE id = $3
RETURNING health_score
`
	var s int16
	err := br.r.pool.QueryRow(ctx, q, delta, errCode, id).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return s, err
}

// AddUsage adds the per-call retail cost (cents) to the binding's daily
// running total. Caller is expected to do this in the same transaction
// as the metering.call_logs insert.
func (br *BindingRepo) AddUsage(ctx context.Context, id int64, cents int64) error {
	const q = `
UPDATE pool.credential_services
SET daily_used_cents = daily_used_cents + $1,
    quota_used_cents = quota_used_cents + $1,
    updated_at       = NOW()
WHERE id = $2
`
	tag, err := br.r.pool.Exec(ctx, q, cents, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus flips the binding's status (active / cooldown /
// rate_limited / banned / archived). For cooldown, also sets
// cooldown_until — pass zero time.Time to clear.
func (br *BindingRepo) UpdateStatus(ctx context.Context, id int64, status string, until time.Time) error {
	const q = `
UPDATE pool.credential_services
SET status         = $1,
    cooldown_until = CASE WHEN $1='cooldown' THEN $2 ELSE NULL END,
    updated_at     = NOW()
WHERE id = $3
`
	var u any
	if !until.IsZero() {
		u = until
	}
	tag, err := br.r.pool.Exec(ctx, q, status, u, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanBindings(rows pgx.Rows) ([]ServiceBinding, error) {
	var out []ServiceBinding
	for rows.Next() {
		var b ServiceBinding
		if err := rows.Scan(
			&b.ID, &b.CredentialID, &b.Capability, &b.Tier,
			&b.QPSLimit, &b.DailyLimitCents, &b.QuotaTotalCents,
			&b.DailyUsedCents, &b.QuotaUsedCents, &b.QuotaResetAt,
			&b.CostBasisCents, &b.HealthScore, &b.ConsecutiveFailures,
			&b.LastSuccessAt, &b.LastErrorAt, &b.LastErrorCode,
			&b.Status, &b.CooldownUntil,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
