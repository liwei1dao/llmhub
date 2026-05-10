package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/llmhub/llmhub/internal/events"
	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/worker"
)

type fakeSink struct {
	mu      sync.Mutex
	rows    []events.CallCompleted
	failNth int // 0 disables; >0 = fail on every Nth call (1-based)
	calls   int
}

func (f *fakeSink) InsertCallLog(_ context.Context, ev events.CallCompleted) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failNth > 0 && f.calls%f.failNth == 0 {
		return errors.New("boom")
	}
	f.rows = append(f.rows, ev)
	return nil
}

func TestHandleMessagePersistsValidPayloads(t *testing.T) {
	t.Parallel()
	sink := &fakeSink{}
	agg := &worker.Aggregator{Logger: log.New("test"), Sink: sink}
	for i := 0; i < 3; i++ {
		b, _ := json.Marshal(events.CallCompleted{
			RequestID: "r", UserID: int64(i), Status: "success", BilledCents: int64(i),
		})
		agg.HandleMessage(context.Background(), b)
	}
	if got := agg.Processed(); got != 3 {
		t.Fatalf("processed=%d want 3", got)
	}
	if len(sink.rows) != 3 {
		t.Fatalf("sink rows=%d want 3", len(sink.rows))
	}
}

func TestHandleMessageIgnoresGarbage(t *testing.T) {
	t.Parallel()
	sink := &fakeSink{}
	agg := &worker.Aggregator{Logger: log.New("test"), Sink: sink}
	agg.HandleMessage(context.Background(), []byte("not-json"))
	if agg.Processed() != 0 || sink.calls != 0 {
		t.Fatal("garbage payload should not increment processed nor reach the sink")
	}
}

func TestHandleMessageCountsFailures(t *testing.T) {
	t.Parallel()
	sink := &fakeSink{failNth: 1} // fail every call
	agg := &worker.Aggregator{Logger: log.New("test"), Sink: sink}
	for i := 0; i < 4; i++ {
		b, _ := json.Marshal(events.CallCompleted{RequestID: "r", UserID: int64(i)})
		agg.HandleMessage(context.Background(), b)
	}
	if agg.Processed() != 4 {
		t.Fatalf("processed=%d", agg.Processed())
	}
	if agg.Failed() != 4 {
		t.Fatalf("failed=%d", agg.Failed())
	}
}
