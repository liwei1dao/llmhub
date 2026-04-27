package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// AccountInsert carries everything the admin UI collects when
// onboarding a new upstream account. Credentials go through Vault; the
// DB only stores the reference.
type AccountInsert struct {
	ProviderID            string
	Tier                  string
	Origin                string
	SupportedCapabilities []string
	QuotaTotalCents       int64
	DailyLimitCents       int64
	QPSLimit              int32
	CostBasisCents        int64
	Tags                  []string
	IsolationGroupID      *int64
	APIKeyRef             string
	APIKeyScope           string
}

// AdminAccount is the verbose admin-view of a pool account, joining
// in the primary active API key so listings don't need N+1 queries.
type AdminAccount struct {
	ID                  int64
	ProviderID          string
	Tier                string
	Status              string
	HealthScore         int
	Origin              string
	QuotaTotalCents     int64
	QuotaUsedCents      int64
	DailyLimitCents     int64
	QPSLimit            int32
	CostBasisCents      int64
	Tags                []string
	SupportedCapabilities []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastUsedAt          *time.Time
}

// CreateAdminAccount provisions a fresh pool account plus one API key.
func (r *Repo) CreateAdminAccount(ctx context.Context, in AccountInsert) (int64, error) {
	var accountID int64
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		const insAcc = `
INSERT INTO pool.accounts (provider_id, tier, origin, supported_capabilities,
                           quota_total_cents, daily_limit_cents, qps_limit,
                           cost_basis_cents, tags, isolation_group_id,
                           status, health_score)
VALUES ($1, $2, $3, $4, NULLIF($5, 0), NULLIF($6, 0), $7, $8, $9, $10, 'warmup', 60)
RETURNING id
`
		if err := tx.QueryRow(ctx, insAcc,
			in.ProviderID, in.Tier, in.Origin, in.SupportedCapabilities,
			in.QuotaTotalCents, in.DailyLimitCents, in.QPSLimit,
			in.CostBasisCents, in.Tags, in.IsolationGroupID,
		).Scan(&accountID); err != nil {
			return err
		}
		if in.APIKeyRef == "" {
			return nil
		}
		const insKey = `
INSERT INTO pool.api_keys (account_id, vault_ref, scope, status)
VALUES ($1, $2, $3, 'active')
`
		_, err := tx.Exec(ctx, insKey, accountID, in.APIKeyRef, nonEmpty(in.APIKeyScope, "chat"))
		return err
	})
	return accountID, err
}

// AccountFilter narrows admin listings.
type AccountFilter struct {
	ProviderID string
	Tier       string
	Status     string
}

// ListAdminAccounts returns accounts matching the filter, newest first.
func (r *Repo) ListAdminAccounts(ctx context.Context, f AccountFilter, limit int) ([]AdminAccount, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const sql = `
SELECT id, provider_id, tier, status, health_score, origin,
       COALESCE(quota_total_cents, 0), quota_used_cents,
       COALESCE(daily_limit_cents, 0), qps_limit, cost_basis_cents,
       COALESCE(tags, '{}'::text[]),
       COALESCE(supported_capabilities, '{}'::text[]),
       created_at, updated_at, last_used_at
FROM pool.accounts
WHERE ($1 = '' OR provider_id = $1)
  AND ($2 = '' OR tier = $2)
  AND ($3 = '' OR status = $3)
ORDER BY created_at DESC
LIMIT $4
`
	rows, err := r.pool.Query(ctx, sql, f.ProviderID, f.Tier, f.Status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminAccount
	for rows.Next() {
		var a AdminAccount
		if err := rows.Scan(
			&a.ID, &a.ProviderID, &a.Tier, &a.Status, &a.HealthScore, &a.Origin,
			&a.QuotaTotalCents, &a.QuotaUsedCents, &a.DailyLimitCents, &a.QPSLimit, &a.CostBasisCents,
			&a.Tags, &a.SupportedCapabilities, &a.CreatedAt, &a.UpdatedAt, &a.LastUsedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAdminAccount returns one account by id, or ErrNotFound.
func (r *Repo) GetAdminAccount(ctx context.Context, id int64) (*AdminAccount, error) {
	const sql = `
SELECT id, provider_id, tier, status, health_score, origin,
       COALESCE(quota_total_cents, 0), quota_used_cents,
       COALESCE(daily_limit_cents, 0), qps_limit, cost_basis_cents,
       COALESCE(tags, '{}'::text[]),
       COALESCE(supported_capabilities, '{}'::text[]),
       created_at, updated_at, last_used_at
FROM pool.accounts WHERE id = $1
`
	var a AdminAccount
	err := r.pool.QueryRow(ctx, sql, id).Scan(
		&a.ID, &a.ProviderID, &a.Tier, &a.Status, &a.HealthScore, &a.Origin,
		&a.QuotaTotalCents, &a.QuotaUsedCents, &a.DailyLimitCents, &a.QPSLimit, &a.CostBasisCents,
		&a.Tags, &a.SupportedCapabilities, &a.CreatedAt, &a.UpdatedAt, &a.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

// AdminPatch carries the fields an admin may update inline.
type AdminPatch struct {
	Tier            *string
	QuotaTotalCents *int64
	DailyLimitCents *int64
	QPSLimit        *int32
	Tags            []string
}

// PatchAdminAccount applies a partial update; zero fields are ignored.
func (r *Repo) PatchAdminAccount(ctx context.Context, id int64, p AdminPatch) error {
	const sql = `
UPDATE pool.accounts
SET tier             = COALESCE($2, tier),
    quota_total_cents = COALESCE($3, quota_total_cents),
    daily_limit_cents = COALESCE($4, daily_limit_cents),
    qps_limit        = COALESCE($5, qps_limit),
    tags             = COALESCE($6, tags),
    updated_at       = NOW()
WHERE id = $1
`
	_, err := r.pool.Exec(ctx, sql, id, p.Tier, p.QuotaTotalCents, p.DailyLimitCents, p.QPSLimit, p.Tags)
	return err
}

// ArchiveAccount sets the lifecycle state to archived and closes active keys.
func (r *Repo) ArchiveAccount(ctx context.Context, id int64, reason string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE pool.accounts SET status = 'archived', updated_at = NOW() WHERE id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE pool.api_keys SET status = 'revoked' WHERE account_id = $1 AND status = 'active'`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO pool.account_events (account_id, event_type, from_state, to_state, reason) VALUES ($1, 'state_change', '', 'archived', $2)`,
			id, reason)
		return err
	})
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
