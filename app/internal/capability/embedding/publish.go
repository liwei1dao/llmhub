package embedding

import (
	"context"
	"encoding/json"
	"time"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/events"
)

// completedPayload mirrors chat.completedPayload but lives in this
// package so the embedding handler stays self-contained.
type completedPayload struct {
	RequestID   string
	UserID      int64
	ModelID     string
	ProviderID  string
	AccountID   int64
	Status      string
	Usage       domain.Usage
	BilledCents int64
}

func publishCompleted(ctx context.Context, d Deps, p completedPayload) {
	if d.Publisher == nil {
		return
	}
	ev := events.CallCompleted{
		RequestID:    p.RequestID,
		UserID:       p.UserID,
		CapabilityID: "embedding",
		ModelID:      p.ModelID,
		ProviderID:   p.ProviderID,
		AccountID:    p.AccountID,
		Status:       p.Status,
		InputTokens:  p.Usage.InputTokens,
		BilledCents:  p.BilledCents,
		StartedAt:    time.Now().UTC(),
	}
	b, err := json.Marshal(ev)
	if err != nil {
		d.Logger.WarnContext(ctx, "embedding publish marshal failed", "err", err)
		return
	}
	d.Publisher.PublishCallCompleted(ctx, b)
}
