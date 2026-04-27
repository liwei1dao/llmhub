package iam

import (
	"context"
	"errors"
	"time"

	"github.com/llmhub/llmhub/internal/iam/repo"
)

// Service is the IAM orchestrator. Handlers call into this layer rather
// than the repo directly, so password / key logic stays out of the
// transport layer.
type Service struct {
	repo *repo.Repo
}

// NewService constructs an IAM service on top of a repo.
func NewService(r *repo.Repo) *Service { return &Service{repo: r} }

// ErrInvalidCredentials covers both unknown users and bad passwords.
// We deliberately use one error to avoid user enumeration.
var ErrInvalidCredentials = errors.New("iam: invalid credentials")

// RegisterRequest carries the minimum fields to create a user.
type RegisterRequest struct {
	Email       string
	Phone       string
	Password    string
	DisplayName string
}

// Register creates a new user with a hashed password.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*repo.User, error) {
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	var emailPtr, phonePtr *string
	if req.Email != "" {
		emailPtr = &req.Email
	}
	if req.Phone != "" {
		phonePtr = &req.Phone
	}
	if emailPtr == nil && phonePtr == nil {
		return nil, errors.New("iam: email or phone required")
	}
	return s.repo.CreateUser(ctx, emailPtr, phonePtr, hash, req.DisplayName)
}

// LoginByEmail verifies credentials and returns the user.
func (s *Service) LoginByEmail(ctx context.Context, email, password string) (*repo.User, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if err := VerifyPassword(u.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}
	_ = s.repo.TouchLastLogin(ctx, u.ID)
	return u, nil
}

// GetUser is a pass-through helper for handlers.
func (s *Service) GetUser(ctx context.Context, id int64) (*repo.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// CreateAPIKeyRequest is the input for creating a user API key.
type CreateAPIKeyRequest struct {
	UserID        int64
	Name          string
	Scopes        []string
	UsageCapCents *int64
	ExpiresAt     *time.Time
}

// CreateAPIKeyResult carries the one-time plaintext back to the caller.
type CreateAPIKeyResult struct {
	Key   *repo.APIKey
	Plain string // returned once; never persisted or logged
}

// CreateAPIKey generates, hashes, and persists a new API key for the user.
func (s *Service) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*CreateAPIKeyResult, error) {
	gen, err := NewAPIKey()
	if err != nil {
		return nil, err
	}
	k, err := s.repo.CreateAPIKey(ctx, req.UserID, gen.PrefixVisible, gen.Hash, req.Name, req.Scopes, req.UsageCapCents, req.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &CreateAPIKeyResult{Key: k, Plain: gen.Plaintext}, nil
}

// ListAPIKeys returns non-plaintext key metadata for the user's UI.
func (s *Service) ListAPIKeys(ctx context.Context, userID int64) ([]repo.APIKey, error) {
	return s.repo.ListAPIKeysByUser(ctx, userID)
}

// RevokeAPIKey is idempotent.
func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	return s.repo.RevokeAPIKey(ctx, userID, keyID)
}

// AuthenticateAPIKey is the gateway-hot-path lookup: given a plaintext
// key received on the wire, verify its shape, hash it, and return the
// owning row. Returns ErrInvalidCredentials on any mismatch.
func (s *Service) AuthenticateAPIKey(ctx context.Context, plaintext string) (*repo.APIKey, error) {
	hash, err := HashAPIKey(plaintext)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	k, err := s.repo.GetAPIKeyByHash(ctx, hash)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	return k, err
}
