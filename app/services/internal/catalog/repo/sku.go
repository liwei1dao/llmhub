package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SKU is the v0.2 platform_services row joined with the currently
// effective platform_pricing row. The gateway uses it to resolve the
// model id in /v1/chat/completions into routing + pricing in a single
// round-trip.
type SKU struct {
	ID              string
	CategoryID      string
	DisplayName     string
	VendorProductID string
	Capability      string
	UpstreamModel   string
	BillingUnit     string
	ContextWindow   *int
	MaxOutputTokens *int
	Status          string
	IsPublic        bool
	// Currently-effective retail price (may be nil if unpriced).
	InputCents  *float64
	OutputCents *float64
	ImageCents  *float64
	PricingAt   *time.Time
}

// GetSKU resolves a SKU id to its routing + pricing snapshot. Returns
// ErrNotFound if the SKU does not exist (status 'deprecated' rows are
// included; callers decide whether to honor them).
func (r *Repo) GetSKU(ctx context.Context, id string) (*SKU, error) {
	const sql = `
SELECT s.id, s.category_id, s.display_name, s.vendor_product_id,
       s.capability, COALESCE(s.upstream_model, ''),
       s.billing_unit, s.context_window, s.max_output_tokens,
       s.status, s.is_public,
       p.input_per_unit_cents, p.output_per_unit_cents, p.image_per_unit_cents,
       p.effective_from
FROM catalog.platform_services s
LEFT JOIN LATERAL (
  SELECT input_per_unit_cents, output_per_unit_cents, image_per_unit_cents, effective_from
  FROM catalog.platform_pricing
  WHERE platform_service_id = s.id
    AND effective_from <= NOW()
    AND (effective_until IS NULL OR effective_until > NOW())
  ORDER BY effective_from DESC
  LIMIT 1
) p ON TRUE
WHERE s.id = $1
`
	var s SKU
	err := r.pool.QueryRow(ctx, sql, id).Scan(
		&s.ID, &s.CategoryID, &s.DisplayName, &s.VendorProductID,
		&s.Capability, &s.UpstreamModel,
		&s.BillingUnit, &s.ContextWindow, &s.MaxOutputTokens,
		&s.Status, &s.IsPublic,
		&s.InputCents, &s.OutputCents, &s.ImageCents,
		&s.PricingAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}
