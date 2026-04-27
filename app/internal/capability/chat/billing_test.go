package chat

import (
	"testing"

	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/domain"
)

func TestCostFromUsageZeroWhenNoPricing(t *testing.T) {
	t.Parallel()
	if c := CostFromUsage(nil, domain.Usage{InputTokens: 1000}); c != 0 {
		t.Fatalf("expected 0 cents without pricing, got %d", c)
	}
}

func TestCostFromUsageRounds(t *testing.T) {
	t.Parallel()
	p := &catalogrepo.Pricing{
		InputPer1KCents:  0.80,
		OutputPer1KCents: 2.00,
	}
	// 500 in × 0.8/1k = 0.4; 500 out × 2/1k = 1.0; total 1.4; ceil = 2
	got := CostFromUsage(p, domain.Usage{InputTokens: 500, OutputTokens: 500})
	if got != 2 {
		t.Fatalf("expected 2 cents, got %d", got)
	}
}

func TestCostFromUsageMinimumOneCent(t *testing.T) {
	t.Parallel()
	p := &catalogrepo.Pricing{InputPer1KCents: 0, OutputPer1KCents: 0}
	// Pricing all zeros but tokens present → floor to 1 cent minimum.
	got := CostFromUsage(p, domain.Usage{InputTokens: 10, OutputTokens: 5})
	if got != 1 {
		t.Fatalf("expected minimum 1 cent, got %d", got)
	}
}

func TestEstimateFreezeHasFloor(t *testing.T) {
	t.Parallel()
	// Tiny prompt → floor at 100 cents ($1 max burn cap on unpriced requests).
	if got := EstimateFreeze(nil, 10, 100); got < 100 {
		t.Fatalf("expected freeze >= 100 cents, got %d", got)
	}
}

func TestEstimateFreezeScalesWithBudget(t *testing.T) {
	t.Parallel()
	p := &catalogrepo.Pricing{InputPer1KCents: 0.8, OutputPer1KCents: 2}
	small := EstimateFreeze(p, 30, 100)
	large := EstimateFreeze(p, 30, 100000)
	if large <= small {
		t.Fatalf("expected higher max_tokens to drive higher freeze, got small=%d large=%d", small, large)
	}
}
