package chat

import (
	"math"

	"github.com/llmhub/llmhub/internal/catalog/repo"
	"github.com/llmhub/llmhub/internal/domain"
)

// CostFromUsage computes the retail cents (in 1/100 cent precision is
// overkill for chat, so we round up to the nearest integer cent) based
// on catalog pricing. Used by chat handler at settle time.
func CostFromUsage(pricing *repo.Pricing, usage domain.Usage) int64 {
	if pricing == nil {
		return 0
	}
	in := float64(usage.InputTokens) / 1000.0 * pricing.InputPer1KCents
	out := float64(usage.OutputTokens) / 1000.0 * pricing.OutputPer1KCents
	total := in + out
	if total <= 0 && usage.InputTokens+usage.OutputTokens > 0 {
		// Minimum billable unit to cover the scheduling overhead.
		total = 1
	}
	return int64(math.Ceil(total))
}

// EstimateFreeze returns a conservative hold amount based on the
// request's message length and max_tokens. Used at request time.
func EstimateFreeze(pricing *repo.Pricing, inputChars, maxOutputTokens int) int64 {
	// 1 token ≈ 3 characters as a rough mixed-language heuristic.
	inputTokens := inputChars / 3
	in := float64(inputTokens) / 1000.0
	out := float64(maxOutputTokens) / 1000.0
	var cents float64
	if pricing != nil {
		cents = in*pricing.InputPer1KCents + out*pricing.OutputPer1KCents
	}
	// Apply a safety factor so short prompts with unset max_tokens still
	// freeze a meaningful amount.
	cents *= 2
	if cents < 100 {
		cents = 100
	}
	return int64(math.Ceil(cents))
}
