package repo

import "context"

// ListAdminPricing is an admin-view listing of every pricing snapshot,
// most recent first. Operators use this to audit the catalog loader's
// output and to diff across deploys.
func (r *Repo) ListAdminPricing(ctx context.Context, modelID string, limit int) ([]Pricing, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const sql = `
SELECT model_id, provider_id, capability_id, kind, unit,
       input_per_1k_cents, output_per_1k_cents, unit_price_cents,
       effective_from, effective_until
FROM catalog.pricing
WHERE ($1 = '' OR model_id = $1)
ORDER BY effective_from DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, sql, modelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pricing
	for rows.Next() {
		var p Pricing
		if err := rows.Scan(
			&p.ModelID, &p.ProviderID, &p.CapabilityID, &p.Kind, &p.Unit,
			&p.InputPer1KCents, &p.OutputPer1KCents, &p.UnitPriceCents,
			&p.EffectiveFrom, &p.EffectiveUntil,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
