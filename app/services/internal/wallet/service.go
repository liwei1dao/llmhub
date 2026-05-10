package wallet

import (
	"context"
	"errors"

	"github.com/llmhub/llmhub/internal/wallet/repo"
)

// Service is the wallet orchestrator. In the v0.2 聚合 SDK 平台 model
// the wallet records balance + recharges + invoices for buying SKU
// subscriptions; it does *not* freeze/settle on a per-call basis (the
// SDK calls upstream directly, the platform never sees the call).
//
// Per-call freeze/settle was the v0.1 中间站 pattern and lived here as
// Freeze / Settle / Release; it has been removed along with the gateway.
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
