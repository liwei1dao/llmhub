package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// APIKey is the persistent representation of a user's API key.
type APIKey struct {
	ID            int64
	UserID        int64
	Prefix        string
	KeyHash       string
	Name          *string
	Scopes        []string
	Status        string
	UsageCapCents *int64
	UsedCents     int64
	LastUsedAt    *time.Time
	ExpiresAt     *time.Time
	CreatedAt     time.Time
}

// CreateAPIKey inserts a new API key row.
func (r *Repo) CreateAPIKey(ctx context.Context, userID int64, prefix, hash, name string, scopes []string, capCents *int64, expiresAt *time.Time) (*APIKey, error) {
	k := &APIKey{
		UserID:        userID,
		Prefix:        prefix,
		KeyHash:       hash,
		Name:          &name,
		Scopes:        scopes,
		Status:        "active",
		UsageCapCents: capCents,
		ExpiresAt:     expiresAt,
	}
	const q = `
INSERT INTO iam.api_keys (user_id, prefix, key_hash, name, scopes, usage_cap_cents, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at
`
	if err := r.pool.QueryRow(ctx, q, userID, prefix, hash, name, scopes, capCents, expiresAt).
		Scan(&k.ID, &k.CreatedAt); err != nil {
		return nil, err
	}
	return k, nil
}

// ListAPIKeysByUser returns all keys for a user, newest first.
func (r *Repo) ListAPIKeysByUser(ctx context.Context, userID int64) ([]APIKey, error) {
	const q = `
SELECT id, user_id, prefix, key_hash, name, scopes, status, usage_cap_cents,
       used_cents, last_used_at, expires_at, created_at
FROM iam.api_keys
WHERE user_id = $1
ORDER BY created_at DESC
`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Prefix, &k.KeyHash, &k.Name, &k.Scopes, &k.Status,
			&k.UsageCapCents, &k.UsedCents, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetAPIKeyByHash is the auth hot path. Returns ErrNotFound if the
// hash is unknown or the key is not in active status.
func (r *Repo) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	const q = `
SELECT id, user_id, prefix, key_hash, name, scopes, status, usage_cap_cents,
       used_cents, last_used_at, expires_at, created_at
FROM iam.api_keys
WHERE key_hash = $1 AND status = 'active'
`
	var k APIKey
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&k.ID, &k.UserID, &k.Prefix, &k.KeyHash, &k.Name, &k.Scopes, &k.Status,
		&k.UsageCapCents, &k.UsedCents, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// RevokeAPIKey flips the status to revoked. Idempotent.
func (r *Repo) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE iam.api_keys SET status = 'revoked' WHERE id = $1 AND user_id = $2`,
		keyID, userID)
	return err
}
