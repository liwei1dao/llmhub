// Package repo is the data-access layer for the catalog domain
// (capabilities, providers, logical models, model mappings, pricing,
// and logical voices).
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is the catalog-layer missing row sentinel.
var ErrNotFound = errors.New("catalog/repo: not found")

// Repo is the catalog data-access layer.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Mapping resolves a logical model id against a specific provider.
type Mapping struct {
	ModelID       string
	ProviderID    string
	UpstreamModel string
	Priority      int16
	Status        string
}

// GetMapping returns the active mapping for (logical_model, provider).
func (r *Repo) GetMapping(ctx context.Context, modelID, providerID string) (*Mapping, error) {
	const sql = `
SELECT model_id, provider_id, upstream_model, priority, status
FROM catalog.model_mappings
WHERE model_id = $1 AND provider_id = $2 AND status = 'active'
ORDER BY priority ASC
LIMIT 1
`
	var m Mapping
	err := r.pool.QueryRow(ctx, sql, modelID, providerID).Scan(
		&m.ModelID, &m.ProviderID, &m.UpstreamModel, &m.Priority, &m.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

// ListProvidersForModel returns every active provider mapping for a logical model, priority-sorted.
func (r *Repo) ListProvidersForModel(ctx context.Context, modelID string) ([]Mapping, error) {
	const sql = `
SELECT model_id, provider_id, upstream_model, priority, status
FROM catalog.model_mappings
WHERE model_id = $1 AND status = 'active'
ORDER BY priority ASC
`
	rows, err := r.pool.Query(ctx, sql, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mapping
	for rows.Next() {
		var m Mapping
		if err := rows.Scan(&m.ModelID, &m.ProviderID, &m.UpstreamModel, &m.Priority, &m.Status); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Pricing is a single pricing snapshot.
type Pricing struct {
	ModelID           string
	ProviderID        *string
	CapabilityID      *string
	Kind              string  // retail / wholesale
	Unit              string  // token / second / char / image / page
	InputPer1KCents   float64
	OutputPer1KCents  float64
	UnitPriceCents    float64
	EffectiveFrom     time.Time
	EffectiveUntil    *time.Time
}

// GetActivePricing returns the currently-effective pricing row. The
// caller passes kind ("retail" to charge users, "wholesale" to compute
// upstream cost). ProviderID is optional: null means "platform default".
func (r *Repo) GetActivePricing(ctx context.Context, modelID, providerID, capabilityID, kind string) (*Pricing, error) {
	const sql = `
SELECT model_id, provider_id, capability_id, kind, unit,
       input_per_1k_cents, output_per_1k_cents, unit_price_cents,
       effective_from, effective_until
FROM catalog.pricing
WHERE model_id = $1
  AND kind = $2
  AND (provider_id = $3 OR ($3 = '' AND provider_id IS NULL))
  AND (effective_until IS NULL OR effective_until > NOW())
  AND effective_from <= NOW()
ORDER BY effective_from DESC
LIMIT 1
`
	var p Pricing
	err := r.pool.QueryRow(ctx, sql, modelID, kind, providerID).Scan(
		&p.ModelID, &p.ProviderID, &p.CapabilityID, &p.Kind, &p.Unit,
		&p.InputPer1KCents, &p.OutputPer1KCents, &p.UnitPriceCents,
		&p.EffectiveFrom, &p.EffectiveUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}
