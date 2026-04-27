package scheduler

import (
	"testing"

	"github.com/llmhub/llmhub/internal/domain"
)

func TestScorePrefersHigherHealth(t *testing.T) {
	t.Parallel()
	low := ScoreInputs{
		Account: domain.PoolAccount{HealthScore: 30, Tier: domain.TierT2, QuotaTotalCents: 1000, QuotaUsedCents: 500},
	}
	high := ScoreInputs{
		Account: domain.PoolAccount{HealthScore: 90, Tier: domain.TierT2, QuotaTotalCents: 1000, QuotaUsedCents: 500},
	}
	if Score(high, DefaultWeights) <= Score(low, DefaultWeights) {
		t.Fatal("expected higher health score to win")
	}
}

func TestScoreLowRiskPrefersT1(t *testing.T) {
	t.Parallel()
	t1 := ScoreInputs{Account: domain.PoolAccount{HealthScore: 70, Tier: domain.TierT1, QuotaTotalCents: 1000, QuotaUsedCents: 500}, RiskLevel: domain.RiskLow}
	t3 := ScoreInputs{Account: domain.PoolAccount{HealthScore: 70, Tier: domain.TierT3, QuotaTotalCents: 1000, QuotaUsedCents: 500}, RiskLevel: domain.RiskLow}
	if Score(t1, DefaultWeights) <= Score(t3, DefaultWeights) {
		t.Fatal("low-risk traffic should prefer T1 over T3")
	}
}

func TestScoreHighRiskAvoidsT1(t *testing.T) {
	t.Parallel()
	t1 := ScoreInputs{Account: domain.PoolAccount{HealthScore: 70, Tier: domain.TierT1, QuotaTotalCents: 1000, QuotaUsedCents: 500}, RiskLevel: domain.RiskHigh}
	t3 := ScoreInputs{Account: domain.PoolAccount{HealthScore: 70, Tier: domain.TierT3, QuotaTotalCents: 1000, QuotaUsedCents: 500}, RiskLevel: domain.RiskHigh}
	if Score(t3, DefaultWeights) <= Score(t1, DefaultWeights) {
		t.Fatal("high-risk traffic should prefer T3 over T1")
	}
}

func TestScoreT3UrgentWhenNearlyDepleted(t *testing.T) {
	t.Parallel()
	// Two T3 accounts with identical health; the one closer to burn-out should score higher.
	empty := ScoreInputs{Account: domain.PoolAccount{HealthScore: 70, Tier: domain.TierT3, QuotaTotalCents: 1000, QuotaUsedCents: 950}, RiskLevel: domain.RiskHigh}
	full := ScoreInputs{Account: domain.PoolAccount{HealthScore: 70, Tier: domain.TierT3, QuotaTotalCents: 1000, QuotaUsedCents: 50}, RiskLevel: domain.RiskHigh}
	if Score(empty, DefaultWeights) <= Score(full, DefaultWeights) {
		t.Fatal("near-depleted T3 should be preferred for consumption")
	}
}
