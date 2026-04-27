// Package repo is the data-access layer for the pool domain
// (upstream accounts, isolation groups, per-capability quotas,
// lifecycle events).
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/llmhub/llmhub/internal/domain"
)

// ErrNotFound is returned when a pool row is missing.
var ErrNotFound = errors.New("pool/repo: not found")

// Repo is the pool domain data-access layer.
type Repo struct {
	pool *pgxpool.Pool
}

// New constructs a Repo.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// CandidateQuery narrows which pool accounts the scheduler will consider.
type CandidateQuery struct {
	ProviderID   string
	CapabilityID string
	MinHealth    int
	ExcludeIDs   []int64
}

// ListCandidates returns active accounts matching the filter, highest
// health score first. The caller (scheduler) then applies scoring and
// session-stickiness preference.
func (r *Repo) ListCandidates(ctx context.Context, q CandidateQuery) ([]domain.PoolAccount, error) {
	const sql = `
SELECT id, provider_id, tier, status, health_score,
       COALESCE(supported_capabilities, '{}'::text[]),
       COALESCE(quota_total_cents, 0),
       quota_used_cents,
       quota_reset_at,
       qps_limit,
       COALESCE(daily_limit_cents, 0),
       isolation_group_id,
       last_used_at,
       last_error_at,
       consecutive_failures,
       COALESCE(tags, '{}'::text[])
FROM pool.accounts
WHERE status = 'active'
  AND health_score >= $1
  AND provider_id = $2
  AND ($3 = '' OR $3 = ANY(supported_capabilities))
  AND ($4::bigint[] IS NULL OR NOT (id = ANY($4)))
ORDER BY health_score DESC, last_used_at ASC NULLS FIRST
LIMIT 50
`
	rows, err := r.pool.Query(ctx, sql, q.MinHealth, q.ProviderID, q.CapabilityID, q.ExcludeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PoolAccount
	for rows.Next() {
		var a domain.PoolAccount
		var tier, status string
		var quotaReset *time.Time
		var lastUsed, lastErr *time.Time
		var isolation *int64
		if err := rows.Scan(
			&a.ID, &a.ProviderID, &tier, &status, &a.HealthScore,
			&a.SupportedCapabilities,
			&a.QuotaTotalCents, &a.QuotaUsedCents, &quotaReset,
			&a.QPSLimit, &a.DailyLimitCents,
			&isolation, &lastUsed, &lastErr, &a.ConsecutiveFailures, &a.Tags,
		); err != nil {
			return nil, err
		}
		a.Tier = domain.Tier(tier)
		a.State = domain.AccountState(status)
		a.QuotaResetAt = quotaReset
		a.LastUsedAt = lastUsed
		a.LastErrorAt = lastErr
		if isolation != nil {
			a.IsolationGroupID = *isolation
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAccount fetches a single account by id.
func (r *Repo) GetAccount(ctx context.Context, id int64) (*domain.PoolAccount, error) {
	const sql = `
SELECT id, provider_id, tier, status, health_score,
       COALESCE(supported_capabilities, '{}'::text[])
FROM pool.accounts
WHERE id = $1
`
	var a domain.PoolAccount
	var tier, status string
	err := r.pool.QueryRow(ctx, sql, id).Scan(
		&a.ID, &a.ProviderID, &tier, &status, &a.HealthScore, &a.SupportedCapabilities,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Tier = domain.Tier(tier)
	a.State = domain.AccountState(status)
	return &a, nil
}

// AdjustHealth nudges the account's health score and records the event.
// Health is clamped to [0,100]. Returns the new score.
func (r *Repo) AdjustHealth(ctx context.Context, accountID int64, delta int, reason string) (int, error) {
	var newScore int
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		const upd = `
UPDATE pool.accounts
SET health_score = GREATEST(0, LEAST(100, health_score + $1)),
    consecutive_failures = CASE WHEN $1 < 0 THEN consecutive_failures + 1 ELSE 0 END,
    last_error_at = CASE WHEN $1 < 0 THEN NOW() ELSE last_error_at END,
    last_used_at  = NOW(),
    updated_at    = NOW()
WHERE id = $2
RETURNING health_score
`
		if err := tx.QueryRow(ctx, upd, delta, accountID).Scan(&newScore); err != nil {
			return err
		}
		const ev = `
INSERT INTO pool.account_events (account_id, event_type, reason)
VALUES ($1, 'health_update', $2)
`
		_, err := tx.Exec(ctx, ev, accountID, reason)
		return err
	})
	return newScore, err
}

// TransitionState flips an account's lifecycle state and records the event.
func (r *Repo) TransitionState(ctx context.Context, accountID int64, to, reason string) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		var from string
		const load = `SELECT status FROM pool.accounts WHERE id = $1 FOR UPDATE`
		if err := tx.QueryRow(ctx, load, accountID).Scan(&from); err != nil {
			return err
		}
		if from == to {
			return nil
		}
		const upd = `UPDATE pool.accounts SET status = $1, updated_at = NOW() WHERE id = $2`
		if _, err := tx.Exec(ctx, upd, to, accountID); err != nil {
			return err
		}
		const ev = `
INSERT INTO pool.account_events (account_id, event_type, from_state, to_state, reason)
VALUES ($1, 'state_change', $2, $3, $4)
`
		_, err := tx.Exec(ctx, ev, accountID, from, to, reason)
		return err
	})
}

func (r *Repo) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
