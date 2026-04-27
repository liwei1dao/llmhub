package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Session is a web UI login session (distinct from API keys).
type Session struct {
	ID        uuid.UUID
	UserID    int64
	TokenHash string
	UserAgent *string
	IP        *string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

// CreateSession inserts a new session.
func (r *Repo) CreateSession(ctx context.Context, s Session) error {
	const q = `
INSERT INTO iam.sessions (id, user_id, token_hash, user_agent, ip, expires_at)
VALUES ($1, $2, $3, $4, $5::inet, $6)
`
	_, err := r.pool.Exec(ctx, q, s.ID, s.UserID, s.TokenHash, s.UserAgent, s.IP, s.ExpiresAt)
	return err
}

// RevokeSession marks the given session revoked.
func (r *Repo) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE iam.sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// FindSessionByHash returns the session for the given token hash, only
// if it is still live (not expired, not revoked). Returns pgx.ErrNoRows
// when no usable session matches.
func (r *Repo) FindSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	const q = `
SELECT id, user_id, token_hash, user_agent, host(ip), expires_at, created_at, revoked_at
FROM iam.sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
LIMIT 1
`
	var s Session
	if err := r.pool.QueryRow(ctx, q, tokenHash).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.UserAgent, &s.IP, &s.ExpiresAt, &s.CreatedAt, &s.RevokedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
