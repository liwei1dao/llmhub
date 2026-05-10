// Package adminauth is the back-office identity domain.
//
// 后台管理员是 LLMHub 平台的内部运营 / 客服 / 财务账号，与终端用户
// (iam.users) 完全分离的两套用户体系：
//   - 终端用户：消费侧，注册自营销站，有钱包、订阅、API key
//   - 后台管理员：运营侧，员工帐号，账号+密码登录，scope 仅限平台管理
//
// 入口流程：
//   1. main.go 启动时调用 EnsureBootstrap 从环境变量种子化首位管理员
//   2. 浏览器 POST /api/admin/auth/login → service.Login → IssueSession
//   3. 后续请求带 X-Admin-Token: <token> → middleware 调 ResolveSession
//   4. 退出 → POST /auth/logout → RevokeSession
package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/llmhub/llmhub/internal/iam"
	authrepo "github.com/llmhub/llmhub/internal/adminauth/repo"
)

// SessionTTL is how long a login token is valid. Mirrors iam.SessionTTL
// so admin / user UX behave the same; can diverge later if we want
// short-lived admin sessions.
const SessionTTL = 7 * 24 * time.Hour

// ErrInvalidCredentials is returned for bad account / password.
var ErrInvalidCredentials = errors.New("adminauth: invalid credentials")

// ErrAdminDisabled is returned when login matches a disabled admin.
var ErrAdminDisabled = errors.New("adminauth: admin disabled")

// ErrSessionInvalid is returned when a token does not resolve.
var ErrSessionInvalid = errors.New("adminauth: session invalid")

// Service wraps the adminauth repo with login / session helpers.
// It does not own the DB pool; the caller passes a Repo.
type Service struct {
	repo *authrepo.Repo
}

// New constructs a Service from a Repo.
func New(r *authrepo.Repo) *Service { return &Service{repo: r} }

// Repo exposes the underlying repo for handlers that need direct access
// (e.g. listing admins for an admin-management page later). Read-only.
func (s *Service) Repo() *authrepo.Repo { return s.repo }

// Login verifies credentials. Returns ErrInvalidCredentials for both
// "no such account" and "wrong password" so the API doesn't leak which
// of the two failed.
func (s *Service) Login(ctx context.Context, account, password string) (*authrepo.Admin, error) {
	account = strings.TrimSpace(account)
	a, err := s.repo.GetAdminByAccount(ctx, account)
	if errors.Is(err, authrepo.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if a.Status != "active" {
		return nil, ErrAdminDisabled
	}
	if err := iam.VerifyPassword(a.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}
	_ = s.repo.TouchLastLogin(ctx, a.ID)
	return a, nil
}

// IssuedSession is the login result handed back to the browser.
type IssuedSession struct {
	Token     string    // raw token; goes in X-Admin-Token header — not stored server-side
	ID        uuid.UUID
	AdminID   int64
	ExpiresAt time.Time
}

// IssueSession creates and persists a session for the given admin.
// The plain token is returned to the caller exactly once; storage uses
// SHA-256 of the token so a DB leak doesn't yield live sessions.
func (s *Service) IssueSession(ctx context.Context, adminID int64, userAgent, clientIP string) (*IssuedSession, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("session entropy: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	sum := sha256.Sum256([]byte(token))

	sess := authrepo.Session{
		ID:        uuid.New(),
		AdminID:   adminID,
		TokenHash: hex.EncodeToString(sum[:]),
		ExpiresAt: time.Now().Add(SessionTTL),
	}
	if userAgent != "" {
		sess.UserAgent = &userAgent
	}
	if clientIP != "" {
		sess.ClientIP = &clientIP
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	return &IssuedSession{
		Token:     token,
		ID:        sess.ID,
		AdminID:   adminID,
		ExpiresAt: sess.ExpiresAt,
	}, nil
}

// ResolveSession validates a raw token and returns the admin id behind it.
// Returns ErrSessionInvalid for any failure (expired, revoked, unknown).
func (s *Service) ResolveSession(ctx context.Context, token string) (int64, error) {
	if token == "" {
		return 0, ErrSessionInvalid
	}
	sum := sha256.Sum256([]byte(token))
	sess, err := s.repo.FindLiveSessionByHash(ctx, hex.EncodeToString(sum[:]))
	if errors.Is(err, authrepo.ErrNotFound) {
		return 0, ErrSessionInvalid
	}
	if err != nil {
		return 0, err
	}
	return sess.AdminID, nil
}

// RevokeSessionByToken revokes the session that matches the given raw token.
// No-op if the token does not match a live session.
func (s *Service) RevokeSessionByToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(token))
	sess, err := s.repo.FindLiveSessionByHash(ctx, hex.EncodeToString(sum[:]))
	if errors.Is(err, authrepo.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.repo.RevokeSession(ctx, sess.ID)
}

// EnsureBootstrap creates the first admin from env-supplied credentials
// when adminauth.admins is empty. No-op when at least one admin exists,
// so re-running on every startup is safe.
//
// account / password may be empty; in that case the function silently
// returns nil. main.go is the only expected caller.
func (s *Service) EnsureBootstrap(ctx context.Context, account, password, displayName string) error {
	account = strings.TrimSpace(account)
	if account == "" || password == "" {
		return nil
	}
	n, err := s.repo.CountAdmins(ctx)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if n > 0 {
		return nil
	}
	hash, err := iam.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if _, err := s.repo.CreateAdmin(ctx, account, hash, displayName); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	return nil
}
