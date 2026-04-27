package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/events"
)

// completedPayload is the gateway's eventing-time view of a call. Kept
// internal so the handler stays in control of which fields ship.
type completedPayload struct {
	RequestID   string
	UserID      int64
	Capability  string
	ModelID     string
	ProviderID  string
	AccountID   int64
	Status      string
	Usage       domain.Usage
	BilledCents int64
	StartedAt   time.Time
	DurationMs  int
}

// publishCallCompleted serializes the payload into the events.CallCompleted
// schema and hands it to the publisher. nil publisher is a no-op.
func publishCallCompleted(ctx context.Context, d Deps, p completedPayload) {
	if d.Publisher == nil {
		return
	}
	if p.StartedAt.IsZero() {
		p.StartedAt = time.Now().UTC()
	}
	ev := events.CallCompleted{
		RequestID:    p.RequestID,
		UserID:       p.UserID,
		CapabilityID: p.Capability,
		ModelID:      p.ModelID,
		ProviderID:   p.ProviderID,
		AccountID:    p.AccountID,
		Status:       p.Status,
		InputTokens:  p.Usage.InputTokens,
		OutputTokens: p.Usage.OutputTokens,
		AudioSeconds: p.Usage.AudioSeconds,
		Characters:   p.Usage.Characters,
		BilledCents:  p.BilledCents,
		DurationMs:   p.DurationMs,
		StartedAt:    p.StartedAt,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		d.Logger.WarnContext(ctx, "publish call.completed marshal failed", "err", err)
		return
	}
	d.Publisher.PublishCallCompleted(ctx, b)
}
