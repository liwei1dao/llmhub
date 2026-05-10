package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/llmhub/llmhub/internal/platform/log"
)

type stubReconStore struct {
	calls []time.Time
	err   error
}

func (s *stubReconStore) RunDailyRecon(_ context.Context, day time.Time) (int64, error) {
	s.calls = append(s.calls, day)
	if s.err != nil {
		return 0, s.err
	}
	return int64(len(s.calls)), nil
}

func TestDailyReconRunOnceCountsSuccesses(t *testing.T) {
	t.Parallel()
	store := &stubReconStore{}
	d := &DailyRecon{Logger: log.New("test"), Store: store}
	d.runOnce(context.Background(), time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC))
	if d.Runs() != 1 {
		t.Fatalf("runs=%d want 1", d.Runs())
	}
	if len(store.calls) != 1 || store.calls[0].Day() != 23 {
		t.Fatalf("unexpected calls: %v", store.calls)
	}
}

func TestDailyReconRunOnceSkipsCounterOnError(t *testing.T) {
	t.Parallel()
	store := &stubReconStore{err: errors.New("boom")}
	d := &DailyRecon{Logger: log.New("test"), Store: store}
	d.runOnce(context.Background(), time.Now().UTC())
	if d.Runs() != 0 {
		t.Fatalf("runs=%d want 0 on error", d.Runs())
	}
}

func TestNextRunRollsForward(t *testing.T) {
	t.Parallel()
	d := &DailyRecon{HourUTC: 3, MinuteUTC: 30}
	now := time.Date(2026, 4, 24, 5, 0, 0, 0, time.UTC) // already past 03:30 today
	got := d.nextRun(now)
	want := time.Date(2026, 4, 25, 3, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("nextRun=%s want %s", got, want)
	}
}

func TestNextRunPicksTodayIfFuture(t *testing.T) {
	t.Parallel()
	d := &DailyRecon{HourUTC: 12, MinuteUTC: 0}
	now := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	got := d.nextRun(now)
	want := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("nextRun=%s want %s", got, want)
	}
}
