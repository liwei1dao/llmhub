package worker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// DailyReconStore is the data-access boundary for the recon cron.
type DailyReconStore interface {
	RunDailyRecon(ctx context.Context, day time.Time) (int64, error)
}

// DailyRecon snapshots call_logs into metering.reconciliation once a
// day. The cron runs roughly at the configured time-of-day in UTC and
// always processes "yesterday" so it doesn't race writers still
// finishing today's events.
type DailyRecon struct {
	Logger      *slog.Logger
	Store       DailyReconStore
	HourUTC     int // 0..23; default 0 (midnight)
	MinuteUTC   int // 0..59; default 5

	runs atomic.Int64
	now  func() time.Time // for tests
}

// Run loops until ctx is canceled, sleeping precisely until the next
// scheduled tick rather than polling minute-by-minute.
func (d *DailyRecon) Run(ctx context.Context) {
	if d.now == nil {
		d.now = func() time.Time { return time.Now().UTC() }
	}
	for {
		next := d.nextRun(d.now())
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			d.runOnce(ctx, next.AddDate(0, 0, -1))
		}
	}
}

func (d *DailyRecon) nextRun(now time.Time) time.Time {
	candidate := time.Date(now.Year(), now.Month(), now.Day(),
		d.HourUTC, d.MinuteUTC, 0, 0, time.UTC)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

func (d *DailyRecon) runOnce(ctx context.Context, day time.Time) {
	rows, err := d.Store.RunDailyRecon(ctx, day)
	if err != nil {
		d.Logger.WarnContext(ctx, "daily recon failed", "day", day.Format("2006-01-02"), "err", err)
		return
	}
	d.runs.Add(1)
	d.Logger.InfoContext(ctx, "daily recon complete",
		"day", day.Format("2006-01-02"), "rows", rows)
}

// Runs returns the lifetime success count.
func (d *DailyRecon) Runs() int64 { return d.runs.Load() }
