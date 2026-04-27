package embedding

import (
	"context"

	"github.com/llmhub/llmhub/internal/capability"
	"github.com/llmhub/llmhub/internal/domain"
)

// ID is the capability identifier registered in catalog.capabilities.
const ID = "embedding"

// Capability implements capability.Capability for embeddings.
type Capability struct{}

func (Capability) ID() string                            { return ID }
func (Capability) BillingUnit() domain.BillingUnit       { return domain.UnitToken }
func (Capability) RequiredTransport() []domain.Transport { return []domain.Transport{domain.TransportHTTP} }

// Estimate uses input character length as a token proxy.
func (Capability) Estimate(_ context.Context, req *domain.EmbeddingRequest) (domain.BillingEstimate, error) {
	if req == nil {
		return domain.BillingEstimate{}, nil
	}
	chars := 0
	for _, s := range req.Input {
		chars += len(s)
	}
	return domain.BillingEstimate{
		Unit:           domain.UnitToken,
		EstimatedUnits: float64(chars / 3),
	}, nil
}

func init() {
	capability.Register(capability.Descriptor{
		ID:          ID,
		Unit:        domain.UnitToken,
		Transports:  []domain.Transport{domain.TransportHTTP},
		Description: "Vector embedding (OpenAI-compatible)",
	})
}
