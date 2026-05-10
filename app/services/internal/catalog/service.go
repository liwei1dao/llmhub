package catalog

import (
	"context"
	"sync"
	"time"

	"github.com/llmhub/llmhub/internal/catalog/repo"
)

// Service provides hot-path catalog lookups (model routing, pricing)
// behind a short-TTL in-process cache. The cache is busted on loader
// reconcile by bumping the generation counter.
type Service struct {
	repo *repo.Repo

	mu          sync.RWMutex
	generation  uint64
	mappings    map[mappingKey]mappingEntry
	pricing     map[pricingKey]pricingEntry
	cacheTTL    time.Duration
}

type mappingKey struct{ model, provider string }
type mappingEntry struct {
	mapping *repo.Mapping
	at      time.Time
}

type pricingKey struct{ model, provider, capability, kind string }
type pricingEntry struct {
	price *repo.Pricing
	at    time.Time
}

// NewService wires a Service on top of a repo.
func NewService(r *repo.Repo) *Service {
	return &Service{
		repo:     r,
		mappings: make(map[mappingKey]mappingEntry),
		pricing:  make(map[pricingKey]pricingEntry),
		cacheTTL: 30 * time.Second,
	}
}

// ResolveUpstreamModel finds the upstream model id for a (logical, provider) pair.
func (s *Service) ResolveUpstreamModel(ctx context.Context, modelID, providerID string) (string, error) {
	k := mappingKey{model: modelID, provider: providerID}
	s.mu.RLock()
	e, ok := s.mappings[k]
	s.mu.RUnlock()
	if ok && time.Since(e.at) < s.cacheTTL {
		return e.mapping.UpstreamModel, nil
	}
	m, err := s.repo.GetMapping(ctx, modelID, providerID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.mappings[k] = mappingEntry{mapping: m, at: time.Now()}
	s.mu.Unlock()
	return m.UpstreamModel, nil
}

// ProvidersForModel returns the priority-ordered mappings for a logical model.
func (s *Service) ProvidersForModel(ctx context.Context, modelID string) ([]repo.Mapping, error) {
	return s.repo.ListProvidersForModel(ctx, modelID)
}

// Pricing returns the effective pricing for a (model, provider, capability, kind) tuple.
func (s *Service) Pricing(ctx context.Context, modelID, providerID, capabilityID, kind string) (*repo.Pricing, error) {
	k := pricingKey{model: modelID, provider: providerID, capability: capabilityID, kind: kind}
	s.mu.RLock()
	e, ok := s.pricing[k]
	s.mu.RUnlock()
	if ok && time.Since(e.at) < s.cacheTTL {
		return e.price, nil
	}
	p, err := s.repo.GetActivePricing(ctx, modelID, providerID, capabilityID, kind)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.pricing[k] = pricingEntry{price: p, at: time.Now()}
	s.mu.Unlock()
	return p, nil
}

// Invalidate drops every cached entry. Call after a loader run.
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mappings = make(map[mappingKey]mappingEntry)
	s.pricing = make(map[pricingKey]pricingEntry)
	s.generation++
}
