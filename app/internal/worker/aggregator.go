// Package worker hosts the background tasks: NATS event consumers
// (usage aggregation, billing reconciliation triggers) and periodic
// jobs (account warmup, daily reconciliation, balance hold reaper).
package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"

	"github.com/nats-io/nats.go"

	"github.com/llmhub/llmhub/internal/events"
)

// CallLogSink is the persistence boundary the aggregator writes
// through. *meteringrepo.Repo satisfies it; tests use an in-memory
// stub so the aggregator's logic stays decoupled from the schema.
type CallLogSink interface {
	InsertCallLog(ctx context.Context, ev events.CallCompleted) error
}

// Aggregator subscribes to call.completed events and persists each
// one to the metering schema. Failures are logged but never propagate
// to the publisher — best-effort is the right semantic for metering.
type Aggregator struct {
	Logger *slog.Logger
	NC     *nats.Conn
	Sink   CallLogSink

	processed atomic.Int64
	failed    atomic.Int64
}

// HandleMessage decodes one event payload, persists it via Sink, and
// updates internal counters. Exposed so tests can drive the aggregator
// without an embedded NATS server.
func (a *Aggregator) HandleMessage(ctx context.Context, data []byte) {
	var ev events.CallCompleted
	if err := json.Unmarshal(data, &ev); err != nil {
		a.Logger.WarnContext(ctx, "aggregator decode failed", "err", err)
		return
	}
	a.processed.Add(1)
	if a.Sink == nil {
		return
	}
	if err := a.Sink.InsertCallLog(ctx, ev); err != nil {
		a.failed.Add(1)
		a.Logger.WarnContext(ctx, "call_log insert failed",
			"err", err, "request_id", ev.RequestID)
	}
}

// Subscribe wires the consumer onto the configured subject. The
// returned subscription is canceled when ctx is done.
func (a *Aggregator) Subscribe(ctx context.Context) (*nats.Subscription, error) {
	sub, err := a.NC.Subscribe(events.SubjectCallCompleted, func(m *nats.Msg) {
		a.HandleMessage(ctx, m.Data)
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		_ = sub.Drain()
	}()
	return sub, nil
}

// Processed / Failed expose counters for /metrics + tests.
func (a *Aggregator) Processed() int64 { return a.processed.Load() }
func (a *Aggregator) Failed() int64    { return a.failed.Load() }
