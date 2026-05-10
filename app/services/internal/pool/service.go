// Package pool owns the upstream account pool: lifecycle transitions,
// health accounting, and the issue-time binding picker. In the v0.2
// 聚合 SDK 平台 model, the pool is the platform's most valuable asset —
// it stores the *real* upstream credentials that the SDK exchanges for
// at request time.
//
// Concepts (one row each):
//
//   - pool.vendor_accounts      — master billing account at one upstream vendor
//   - pool.credentials          — one upstream API key (or AK/SK / ...) under a vendor account
//   - pool.credential_services  — schedulable binding: (credential, capability)
//   - pool.leases               — short-lived issue records minted by /sdk/credentials/issue
//
// Service exposes the orchestration layer; raw queries live in
// internal/pool/repo. The bulk of v0.2 methods (CreateVendorAccount /
// CreateCredential / PickBinding / ...) live in service_v2.go.
package pool

import (
	"github.com/llmhub/llmhub/internal/pool/repo"
)

// Service is the pool orchestrator.
type Service struct {
	repo *repo.Repo
}

// New constructs a Service.
func New(r *repo.Repo) *Service { return &Service{repo: r} }

// Repo exposes the raw data-access layer for packages inside the pool
// domain (e.g. /sdk/usage/report writes lease + binding rows directly).
func (s *Service) Repo() *repo.Repo { return s.repo }
