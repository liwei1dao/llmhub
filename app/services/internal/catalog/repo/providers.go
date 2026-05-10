package repo

import (
	"context"
	"time"
)

// ProviderRow is the verbose admin view of a catalog.providers row.
type ProviderRow struct {
	ID                    string
	DisplayName           string
	BaseURL               string
	AuthMode              string
	ProtocolFamily        string
	Status                string
	SupportedCapabilities []string
	UpdatedAt             time.Time
}

// ListProviders returns every registered provider in alphabetical order.
func (r *Repo) ListProviders(ctx context.Context) ([]ProviderRow, error) {
	const sql = `
SELECT id, display_name, base_url, auth_mode, protocol_family, status,
       COALESCE(supported_capabilities, '{}'::text[]), updated_at
FROM catalog.providers
ORDER BY id ASC
`
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProviderRow
	for rows.Next() {
		var p ProviderRow
		if err := rows.Scan(
			&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthMode, &p.ProtocolFamily, &p.Status,
			&p.SupportedCapabilities, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
