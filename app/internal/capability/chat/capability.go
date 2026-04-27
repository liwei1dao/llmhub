// Package chat is the capability implementation for LLM chat completions.
package chat

import (
	"context"

	"github.com/llmhub/llmhub/internal/capability"
	"github.com/llmhub/llmhub/internal/domain"
)

// ID is the capability identifier.
const ID = "chat"

// Capability implements capability.Capability[*domain.ChatRequest, *domain.ChatResponse].
type Capability struct{}

func (Capability) ID() string                            { return ID }
func (Capability) BillingUnit() domain.BillingUnit       { return domain.UnitToken }
func (Capability) RequiredTransport() []domain.Transport { return []domain.Transport{domain.TransportHTTP, domain.TransportSSE} }

// Estimate predicts the token spend based on message length + max_tokens.
// TODO(M4): replace the rough heuristic with a tokenizer once available.
func (Capability) Estimate(_ context.Context, req *domain.ChatRequest) (domain.BillingEstimate, error) {
	if req == nil {
		return domain.BillingEstimate{}, nil
	}
	inputChars := 0
	for _, m := range req.Messages {
		if s, ok := m.Content.(string); ok {
			inputChars += len(s)
		}
	}
	// Rough heuristic: 1 token ~= 2 characters for CJK, ~4 for English.
	inputTokens := inputChars / 3
	maxOutput := 2048
	if req.MaxTokens != nil {
		maxOutput = *req.MaxTokens
	}
	return domain.BillingEstimate{
		Unit:           domain.UnitToken,
		EstimatedUnits: float64(inputTokens + maxOutput),
	}, nil
}

func init() {
	capability.Register(capability.Descriptor{
		ID:          ID,
		Unit:        domain.UnitToken,
		Transports:  []domain.Transport{domain.TransportHTTP, domain.TransportSSE},
		Description: "LLM chat completions (OpenAI + Anthropic compatible)",
	})
}
