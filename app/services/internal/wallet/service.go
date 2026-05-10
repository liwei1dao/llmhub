package wallet

import (
	"context"
	"errors"
	"time"

	"github.com/llmhub/llmhub/internal/wallet/repo"
)

// Service is the wallet orchestrator used by the billing service and
// user API. It guarantees every balance mutation is accompanied by a
// transaction row.
type Service struct {
	repo *repo.Repo
}

// NewService returns a wallet service.
func NewService(r *repo.Repo) *Service { return &Service{repo: r} }

// EnsureAccount returns (creating if needed) the user's CNY account.
func (s *Service) EnsureAccount(ctx context.Context, userID int64) (*repo.Account, error) {
	return s.repo.EnsureAccount(ctx, userID, "CNY")
}

// GetAccount returns the user's CNY account; does not create.
func (s *Service) GetAccount(ctx context.Context, userID int64) (*repo.Account, error) {
	a, err := s.repo.GetAccount(ctx, userID, "CNY")
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrAccountNotFound
	}
	return a, err
}

// Default TTL for a freeze; overridable via options in the future.
const defaultHoldTTL = 15 * time.Minute

// Freeze reserves amountCents for the duration of an in-flight call.
func (s *Service) Freeze(ctx context.Context, requestID string, userID, amountCents int64) error {
	if amountCents <= 0 {
		return ErrNegativeAmount
	}
	acc, err := s.EnsureAccount(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.repo.Freeze(ctx, requestID, userID, acc.ID, amountCents, defaultHoldTTL); err != nil {
		if errors.Is(err, repo.ErrInsufficient) {
			return ErrInsufficientFunds
		}
		return err
	}
	return nil
}

// Settle finalizes a hold by applying the actual cost.
func (s *Service) Settle(ctx context.Context, requestID string, actualCents int64) error {
	return s.repo.Settle(ctx, requestID, actualCents)
}

// Release returns a hold's reserved amount with zero charge.
func (s *Service) Release(ctx context.Context, requestID string) error {
	return s.repo.Release(ctx, requestID)
}
