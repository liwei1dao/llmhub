package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/pool"
)

// PickRequest is the scheduler's input.
type PickRequest struct {
	RequestID    string
	UserID       int64
	CapabilityID string
	ProviderID   string // optional hint; in M3 we honor it as-is
	ModelID      string
	RiskLevel    domain.RiskLevel
	EstimatedUnits int
	SessionKey   string // for stickiness
	ExcludeAccountIDs []int64
}

// PickResult is a selected account from the pool.
type PickResult struct {
	AccountID    int64
	ProviderID   string
	Tier         domain.Tier
	PickToken    string // echoed back with Report so we can track retries
}

// Stickiness maps a (user, provider) session to a previously chosen
// account for a bounded duration, to avoid triggering multi-IP / multi-
// account risk heuristics on the upstream side.
type Stickiness interface {
	Get(key string) (int64, bool)
	Put(key string, accountID int64, ttl time.Duration)
}

// MemStickiness is a process-local implementation. M4 swaps to Redis.
type MemStickiness struct {
	mu   sync.RWMutex
	data map[string]stickEntry
}

type stickEntry struct {
	accountID int64
	expiresAt time.Time
}

// NewMemStickiness returns an empty sticky map.
func NewMemStickiness() *MemStickiness {
	return &MemStickiness{data: make(map[string]stickEntry)}
}

func (m *MemStickiness) Get(key string) (int64, bool) {
	m.mu.RLock()
	e, ok := m.data[key]
	m.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return 0, false
	}
	return e.accountID, true
}

func (m *MemStickiness) Put(key string, id int64, ttl time.Duration) {
	m.mu.Lock()
	m.data[key] = stickEntry{accountID: id, expiresAt: time.Now().Add(ttl)}
	m.mu.Unlock()
}

// Picker is the selection engine used by the scheduler Service.
type Picker struct {
	pool       *pool.Service
	sticky     Stickiness
	weights    Weights
	stickyTTL  time.Duration
	minHealth  int
}

// NewPicker wires a picker against the pool service.
func NewPicker(p *pool.Service, sticky Stickiness) *Picker {
	return &Picker{
		pool:      p,
		sticky:    sticky,
		weights:   DefaultWeights,
		stickyTTL: 10 * time.Minute,
		minHealth: 40,
	}
}

// Pick returns the best candidate, honoring session stickiness first.
func (p *Picker) Pick(ctx context.Context, req PickRequest) (*PickResult, *domain.UnifiedError) {
	if key := stickyKey(req); key != "" && !contains(req.ExcludeAccountIDs) {
		if aid, ok := p.sticky.Get(key); ok {
			if a, err := p.pool.Get(ctx, aid); err == nil && a.IsSchedulable() && a.SupportsCapability(req.CapabilityID) {
				return &PickResult{
					AccountID:  a.ID,
					ProviderID: a.ProviderID,
					Tier:       a.Tier,
					PickToken:  req.RequestID,
				}, nil
			}
		}
	}

	cands, err := p.pool.Candidates(ctx, pool.CandidateQuery{
		ProviderID:   req.ProviderID,
		CapabilityID: req.CapabilityID,
		MinHealth:    p.minHealth,
		ExcludeIDs:   req.ExcludeAccountIDs,
	})
	if err != nil {
		return nil, domain.NewError(domain.ErrInternal, "LLMH_500_SCHED", "candidate lookup failed").WithCause(err)
	}
	if len(cands) == 0 {
		return nil, domain.NewError(domain.ErrNoAccountAvailable, "LLMH_503_001", "no upstream account available")
	}

	bestIdx := 0
	bestScore := Score(ScoreInputs{Account: cands[0], RiskLevel: req.RiskLevel}, p.weights)
	for i := 1; i < len(cands); i++ {
		s := Score(ScoreInputs{Account: cands[i], RiskLevel: req.RiskLevel}, p.weights)
		if s > bestScore {
			bestIdx, bestScore = i, s
		}
	}
	chosen := cands[bestIdx]
	if key := stickyKey(req); key != "" {
		p.sticky.Put(key, chosen.ID, p.stickyTTL)
	}
	return &PickResult{
		AccountID:  chosen.ID,
		ProviderID: chosen.ProviderID,
		Tier:       chosen.Tier,
		PickToken:  req.RequestID,
	}, nil
}

func stickyKey(req PickRequest) string {
	if req.SessionKey == "" {
		return ""
	}
	return req.SessionKey + "|" + req.ProviderID + "|" + req.CapabilityID
}

func contains[T comparable](s []T) bool { return len(s) > 0 }
