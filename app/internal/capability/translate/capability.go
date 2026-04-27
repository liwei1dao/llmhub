// Package translate is the capability implementation for text translation.
package translate

import (
	"context"

	"github.com/llmhub/llmhub/internal/capability"
	"github.com/llmhub/llmhub/internal/domain"
)

const ID = "translate_text"

// Capability implements capability.Capability for text translation.
type Capability struct{}

func (Capability) ID() string                      { return ID }
func (Capability) BillingUnit() domain.BillingUnit { return domain.UnitChar }
func (Capability) RequiredTransport() []domain.Transport {
	return []domain.Transport{domain.TransportHTTP}
}

// Estimate sums the input character count across all texts.
func (Capability) Estimate(_ context.Context, req *domain.TranslateRequest) (domain.BillingEstimate, error) {
	if req == nil {
		return domain.BillingEstimate{}, nil
	}
	var chars int
	for _, t := range req.Texts {
		chars += len(t)
	}
	return domain.BillingEstimate{
		Unit:           domain.UnitChar,
		EstimatedUnits: float64(chars),
	}, nil
}

func init() {
	capability.Register(capability.Descriptor{
		ID:          ID,
		Unit:        domain.UnitChar,
		Transports:  []domain.Transport{domain.TransportHTTP},
		Description: "Text translation (MT or LLM-based)",
	})
}
