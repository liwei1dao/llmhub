package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// APIKey is a single credential entry under a pool account.
type APIKey struct {
	ID                  int64
	AccountID           int64
	VaultRef            string
	Scope               string
	UpstreamModelFilter []string
	Status              string
}

// GetActiveAPIKey returns the first active key for the account whose
// scope matches the requested capability (or "all").
func (r *Repo) GetActiveAPIKey(ctx context.Context, accountID int64, capability string) (*APIKey, error) {
	const q = `
SELECT id, account_id, vault_ref, scope,
       COALESCE(upstream_model_filter, '{}'::text[]),
       status
FROM pool.api_keys
WHERE account_id = $1
  AND status = 'active'
  AND (scope = $2 OR scope = 'all')
ORDER BY rotated_at DESC NULLS LAST, id DESC
LIMIT 1
`
	var k APIKey
	err := r.pool.QueryRow(ctx, q, accountID, capability).Scan(
		&k.ID, &k.AccountID, &k.VaultRef, &k.Scope, &k.UpstreamModelFilter, &k.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &k, err
}
