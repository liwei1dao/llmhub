// Package asr is the capability implementation for automatic speech
// recognition (both batch and streaming).
package asr

import (
	"context"

	"github.com/llmhub/llmhub/internal/capability"
	"github.com/llmhub/llmhub/internal/domain"
)

const ID = "asr"

// Capability implements capability.Capability for ASR.
type Capability struct{}

func (Capability) ID() string                      { return ID }
func (Capability) BillingUnit() domain.BillingUnit { return domain.UnitSecond }
func (Capability) RequiredTransport() []domain.Transport {
	return []domain.Transport{domain.TransportHTTP, domain.TransportWebSocket}
}

// Estimate returns a placeholder estimate; batch requests fill in the
// actual duration after upload is complete (caller recomputes).
func (Capability) Estimate(_ context.Context, _ *domain.ASRRequest) (domain.BillingEstimate, error) {
	return domain.BillingEstimate{
		Unit:           domain.UnitSecond,
		EstimatedUnits: 60, // assume 1 min by default before we know size
	}, nil
}

func init() {
	capability.Register(capability.Descriptor{
		ID:          ID,
		Unit:        domain.UnitSecond,
		Transports:  []domain.Transport{domain.TransportHTTP, domain.TransportWebSocket},
		Description: "Automatic speech recognition (batch + streaming)",
	})
}
