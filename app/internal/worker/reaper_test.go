package worker_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/llmhub/llmhub/internal/platform/log"
	"github.com/llmhub/llmhub/internal/worker"
)

type fakeHolds struct {
	mu    sync.Mutex
	rows  []worker.ExpiredHold
	marks []string
	err   error
}

func (f *fakeHolds) ListExpiredHolds(_ context.Context, _ int) ([]worker.ExpiredHold, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := f.rows
	f.rows = nil // single-shot per sweep
	return out, nil
}

func (f *fakeHolds) MarkHoldExpired(_ context.Context, requestID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, requestID)
	return nil
}

type fakeReleaser struct {
	mu        sync.Mutex
	released  []string
	failOnReq string
}

func (f *fakeReleaser) Release(_ context.Context, requestID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if requestID == f.failOnReq {
		return errors.New("boom")
	}
	f.released = append(f.released, requestID)
	return nil
}

func TestReaperReleasesEachExpiredHold(t *testing.T) {
	t.Parallel()
	hs := &fakeHolds{rows: []worker.ExpiredHold{
		{RequestID: "r1", AmountCents: 5},
		{RequestID: "r2", AmountCents: 7},
	}}
	br := &fakeReleaser{}
	r := &worker.HoldReaper{Logger: log.New("test"), Holds: hs, Billing: br}

	// Drive a single sweep manually so the test stays fast.
	worker.SweepForTest(r, context.Background())
	if got := r.Released(); got != 2 {
		t.Fatalf("released=%d want 2", got)
	}
	if len(br.released) != 2 || len(hs.marks) != 2 {
		t.Fatalf("releaser=%v marks=%v", br.released, hs.marks)
	}
}

func TestReaperSkipsBillingFailures(t *testing.T) {
	t.Parallel()
	hs := &fakeHolds{rows: []worker.ExpiredHold{
		{RequestID: "ok", AmountCents: 1},
		{RequestID: "boom", AmountCents: 2},
	}}
	br := &fakeReleaser{failOnReq: "boom"}
	r := &worker.HoldReaper{Logger: log.New("test"), Holds: hs, Billing: br}
	worker.SweepForTest(r, context.Background())
	if got := r.Released(); got != 1 {
		t.Fatalf("released=%d want 1", got)
	}
	if len(hs.marks) != 1 || hs.marks[0] != "ok" {
		t.Fatalf("unexpected marks: %v", hs.marks)
	}
}
