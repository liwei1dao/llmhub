package gateway

import (
	"context"

	"github.com/nats-io/nats.go"

	"github.com/llmhub/llmhub/internal/events"
)

// NATSPublisher implements chat.EventPublisher backed by a NATS conn.
// nil NC is allowed (turns into a no-op publisher).
type NATSPublisher struct {
	NC *nats.Conn
}

// PublishCallCompleted fires the payload at the call.completed subject.
// Errors are swallowed: dropping a metering event must never affect the
// user-visible response.
func (p NATSPublisher) PublishCallCompleted(_ context.Context, payload []byte) {
	if p.NC == nil {
		return
	}
	_ = p.NC.Publish(events.SubjectCallCompleted, payload)
}
