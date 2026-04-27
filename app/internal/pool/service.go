// Package pool owns the upstream account pool: lifecycle transitions,
// health accounting, candidate queries for the scheduler, isolation
// groups, and per-capability quota tracking.
package pool

import (
	"context"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/pool/repo"
)

// Service is the pool orchestrator surface exposed to the scheduler
// and admin APIs.
type Service struct {
	repo *repo.Repo
}

// Repo exposes the raw data-access layer for packages inside the pool
// domain (e.g. the Prober writes probe events directly).
func (s *Service) Repo() *repo.Repo { return s.repo }

// New constructs a Service.
func New(r *repo.Repo) *Service { return &Service{repo: r} }

// CandidateQuery is re-exported so callers don't need to import the repo.
type CandidateQuery = repo.CandidateQuery

// Candidates returns up to N active accounts matching the query.
func (s *Service) Candidates(ctx context.Context, q CandidateQuery) ([]domain.PoolAccount, error) {
	return s.repo.ListCandidates(ctx, q)
}

// Get returns a single account by id.
func (s *Service) Get(ctx context.Context, id int64) (*domain.PoolAccount, error) {
	return s.repo.GetAccount(ctx, id)
}

// ActiveAPIKey returns an active credential entry for the account.
// Re-exports repo.APIKey so callers don't import the repo package.
type APIKey = repo.APIKey

// ActiveAPIKey resolves a credential entry for the given capability scope.
func (s *Service) ActiveAPIKey(ctx context.Context, accountID int64, capability string) (*APIKey, error) {
	return s.repo.GetActiveAPIKey(ctx, accountID, capability)
}

// ReportSuccess bumps an account's health.
func (s *Service) ReportSuccess(ctx context.Context, id int64) error {
	_, err := s.repo.AdjustHealth(ctx, id, +1, "call_success")
	return err
}

// ReportFailure drops health and optionally transitions state.
// `reason` maps roughly to the upstream HTTP status class.
func (s *Service) ReportFailure(ctx context.Context, id int64, reason string) error {
	delta := -5
	switch reason {
	case "rate_limited":
		delta = -10
	case "auth_failed":
		delta = -50
	}
	score, err := s.repo.AdjustHealth(ctx, id, delta, reason)
	if err != nil {
		return err
	}
	// Aggressive transitions — refine in M8 with a proper state machine.
	switch reason {
	case "rate_limited":
		if score < 40 {
			return s.repo.TransitionState(ctx, id, string(domain.AccountCooling), "rate_limited")
		}
	case "auth_failed":
		return s.repo.TransitionState(ctx, id, string(domain.AccountQuarantine), "auth_failed")
	}
	return nil
}
