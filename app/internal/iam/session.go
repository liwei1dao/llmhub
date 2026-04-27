package iam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
)

// SessionTTL is how long a login cookie stays valid.
const SessionTTL = 7 * 24 * time.Hour

// IssuedSession is the result of a login: an opaque token for the
// cookie and the session id (UUID) that it references.
type IssuedSession struct {
	Token     string    // raw value placed in the cookie — never store
	ID        uuid.UUID // primary key in iam.sessions
	UserID    int64
	ExpiresAt time.Time
}

// IssueSession creates and persists a session for the given user. The
// returned Token is the only place the plaintext value exists; only
// its SHA-256 hash is written to the database.
func (s *Service) IssueSession(ctx context.Context, userID int64, userAgent, ip string) (*IssuedSession, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("session entropy: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	sum := sha256.Sum256([]byte(token))

	sess := iamrepo.Session{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hex.EncodeToString(sum[:]),
		UserAgent: ptrOrNil(userAgent),
		IP:        ptrOrNil(ip),
		ExpiresAt: time.Now().Add(SessionTTL),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &IssuedSession{
		Token:     token,
		ID:        sess.ID,
		UserID:    userID,
		ExpiresAt: sess.ExpiresAt,
	}, nil
}

// RevokeSession marks the session revoked.
func (s *Service) RevokeSession(ctx context.Context, id uuid.UUID) error {
	return s.repo.RevokeSession(ctx, id)
}

// ErrInvalidSession covers missing, expired, and revoked sessions.
// Like invalid credentials, we deliberately collapse reasons.
var ErrInvalidSession = errors.New("iam: invalid session")

// ResolveSession validates an incoming cookie token and returns the
// associated user id. The token is hashed and looked up in iam.sessions;
// the row is rejected when expired or revoked, so logout takes effect
// across processes. A Redis hot cache in front of this query lands later.
func (s *Service) ResolveSession(ctx context.Context, token string) (int64, error) {
	if token == "" {
		return 0, ErrInvalidSession
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return 0, ErrInvalidSession
	}
	sum := sha256.Sum256([]byte(token))
	sess, err := s.repo.FindSessionByHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return 0, ErrInvalidSession
	}
	return sess.UserID, nil
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
