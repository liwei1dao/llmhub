package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Session is one logged-in admin browser session.
type Session struct {
	ID         uuid.UUID
	AdminID    int64
	TokenHash  string
	UserAgent  *string
	ClientIP   *string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

// CreateSession inserts a new admin session.
func (r *Repo) CreateSession(ctx context.Context, s Session) error {
	const q = `
INSERT INTO adminauth.sessions (id, admin_id, token_hash, user_agent, client_ip, expires_at)
VALUES ($1, $2, $3, $4, $5::inet, $6)
`
	_, err := r.pool.Exec(ctx, q, s.ID, s.AdminID, s.TokenHash, s.UserAgent, s.ClientIP, s.ExpiresAt)
	return err
}

// FindLiveSessionByHash returns the session for the given token hash
// only if it is not expired and not revoked.
func (r *Repo) FindLiveSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	const q = `
SELECT id, admin_id, token_hash, user_agent, host(client_ip), created_at, expires_at, revoked_at
FROM adminauth.sessions
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
`
	var s Session
	err := r.pool.QueryRow(ctx, q, tokenHash).Scan(
		&s.ID, &s.AdminID, &s.TokenHash, &s.UserAgent, &s.ClientIP,
		&s.CreatedAt, &s.ExpiresAt, &s.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// RevokeSession marks the session as revoked.
func (r *Repo) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE adminauth.sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// PurgeExpiredSessions cleans rows whose expires_at is older than the
// given cutoff. Optional janitor; safe to skip in dev.
func (r *Repo) PurgeExpiredSessions(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM adminauth.sessions WHERE expires_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
