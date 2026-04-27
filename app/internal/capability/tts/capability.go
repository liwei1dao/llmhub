// Package tts is the capability implementation for text-to-speech.
package tts

import (
	"context"

	"github.com/llmhub/llmhub/internal/capability"
	"github.com/llmhub/llmhub/internal/domain"
)

const ID = "tts"

// Capability implements capability.Capability for TTS.
type Capability struct{}

func (Capability) ID() string                      { return ID }
func (Capability) BillingUnit() domain.BillingUnit { return domain.UnitChar }
func (Capability) RequiredTransport() []domain.Transport {
	return []domain.Transport{domain.TransportHTTP, domain.TransportSSE, domain.TransportWebSocket}
}

// Estimate bills by number of input characters.
func (Capability) Estimate(_ context.Context, req *domain.TTSRequest) (domain.BillingEstimate, error) {
	if req == nil {
		return domain.BillingEstimate{}, nil
	}
	return domain.BillingEstimate{
		Unit:           domain.UnitChar,
		EstimatedUnits: float64(len(req.Input)),
	}, nil
}

func init() {
	capability.Register(capability.Descriptor{
		ID:          ID,
		Unit:        domain.UnitChar,
		Transports:  []domain.Transport{domain.TransportHTTP, domain.TransportSSE, domain.TransportWebSocket},
		Description: "Text-to-speech synthesis",
	})
}
