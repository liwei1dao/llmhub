package scheduler

import (
	"context"

	"github.com/llmhub/llmhub/internal/pool"
)

// Service is the public API the gateway talks to. M3 exposes this as a
// Go interface; M4 wraps it in a gRPC server.
type Service struct {
	pool   *pool.Service
	picker *Picker
}

// NewService wires the scheduler to its dependencies.
func NewService(p *pool.Service, picker *Picker) *Service {
	return &Service{pool: p, picker: picker}
}

// Pick selects an upstream account.
func (s *Service) Pick(ctx context.Context, req PickRequest) (*PickResult, error) {
	res, ue := s.picker.Pick(ctx, req)
	if ue != nil {
		return nil, ue
	}
	return res, nil
}

// ReportResult is the call-completion signal fed back by the gateway.
type ReportResult string

const (
	ReportSuccess      ReportResult = "success"
	ReportUpstreamErr  ReportResult = "upstream_error"
	ReportRateLimited  ReportResult = "rate_limited"
	ReportAuthFailed   ReportResult = "auth_failed"
	ReportTimeout      ReportResult = "timeout"
)

// Report updates account health based on the outcome.
func (s *Service) Report(ctx context.Context, accountID int64, r ReportResult) error {
	switch r {
	case ReportSuccess:
		return s.pool.ReportSuccess(ctx, accountID)
	case ReportRateLimited:
		return s.pool.ReportFailure(ctx, accountID, "rate_limited")
	case ReportAuthFailed:
		return s.pool.ReportFailure(ctx, accountID, "auth_failed")
	case ReportTimeout, ReportUpstreamErr:
		return s.pool.ReportFailure(ctx, accountID, "upstream_error")
	}
	return nil
}
