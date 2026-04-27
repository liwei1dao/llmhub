// Package scheduler selects upstream pool accounts for inbound calls
// and receives feedback once calls complete. The scoring formula
// blends health, tier preference, quota urgency, and load balance.
package scheduler

import (
	"github.com/llmhub/llmhub/internal/domain"
)

// ScoreInputs bundles the signals used for a single candidate.
type ScoreInputs struct {
	Account      domain.PoolAccount
	RiskLevel    domain.RiskLevel
	RecentLoad   int // recent QPS on this account (0 if unknown)
}

// Weights control the relative importance of each scoring dimension.
// Exposed as a struct so we can tune it at runtime via the catalog.
type Weights struct {
	Health      float64
	TierMatch   float64
	QuotaUrgent float64
	LoadSpread  float64
}

// DefaultWeights is the M3 starting point. Revisit with live data.
var DefaultWeights = Weights{
	Health:      0.40,
	TierMatch:   0.30,
	QuotaUrgent: 0.20,
	LoadSpread:  0.10,
}

// Score returns a 0..100 score for one candidate. Higher is better.
// Pure function; no I/O — easy to unit test.
func Score(in ScoreInputs, w Weights) float64 {
	h := float64(clamp(in.Account.HealthScore, 0, 100))

	tier := tierAffinity(in.Account.Tier, in.RiskLevel) // 0..100

	remaining := float64(in.Account.QuotaTotalCents - in.Account.QuotaUsedCents)
	total := float64(in.Account.QuotaTotalCents)
	quotaUrgent := 0.0
	// For T3 we want to BURN down near-expired free quota. For T1/T2 we
	// prefer accounts with *plenty* of remaining quota (more stable).
	if total > 0 {
		ratio := remaining / total
		if in.Account.Tier == domain.TierT3 {
			// Closer to zero = more urgent to consume.
			quotaUrgent = 100 * (1 - ratio)
		} else {
			quotaUrgent = 100 * ratio
		}
	}

	load := 100.0 / float64(1+in.RecentLoad) // lower load = higher score

	return w.Health*h + w.TierMatch*tier + w.QuotaUrgent*quotaUrgent + w.LoadSpread*load
}

// tierAffinity rates how well a tier matches a given risk level.
// Low-risk traffic (paid users) prefers higher tiers; high-risk
// (free trial) is fine being served from lower tiers.
func tierAffinity(t domain.Tier, r domain.RiskLevel) float64 {
	switch r {
	case domain.RiskLow:
		switch t {
		case domain.TierT1:
			return 100
		case domain.TierT2:
			return 70
		case domain.TierT4:
			return 60
		case domain.TierT3:
			return 20
		}
	case domain.RiskMedium:
		switch t {
		case domain.TierT2:
			return 100
		case domain.TierT1:
			return 80
		case domain.TierT3:
			return 60
		case domain.TierT4:
			return 50
		}
	case domain.RiskHigh:
		switch t {
		case domain.TierT3:
			return 100
		case domain.TierT2:
			return 60
		case domain.TierT1:
			return 10 // almost never burn expensive enterprise quota on free users
		case domain.TierT4:
			return 30
		}
	}
	return 50
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
