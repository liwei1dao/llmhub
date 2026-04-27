package worker

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// HoldReaper periodically scans metering.balance_holds for entries
// whose TTL has elapsed without a Settle/Release call (e.g. a gateway
// crash mid-call), and asks billing to release them. Without this the
// frozen funds stay locked forever.
type HoldReaper struct {
	Logger  *slog.Logger
	Holds   HoldStore
	Billing HoldReleaser
	Tick    time.Duration
	Batch   int

	released atomic.Int64
}

// HoldStore is the data-access boundary; *meteringrepo.Repo satisfies it.
type HoldStore interface {
	ListExpiredHolds(ctx context.Context, limit int) ([]ExpiredHold, error)
	MarkHoldExpired(ctx context.Context, requestID string) error
}

// HoldReleaser is satisfied by the in-process wallet.Service or the
// remote billing client.
type HoldReleaser interface {
	Release(ctx context.Context, requestID string) error
}

// ExpiredHold is re-exported here so worker callers don't need the
// metering/repo import path.
type ExpiredHold struct {
	RequestID   string
	UserID      int64
	AccountID   int64
	AmountCents int64
}

// Run blocks until ctx is canceled, sweeping every Tick.
func (r *HoldReaper) Run(ctx context.Context) {
	if r.Tick <= 0 {
		r.Tick = 30 * time.Second
	}
	if r.Batch <= 0 {
		r.Batch = 100
	}
	t := time.NewTicker(r.Tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.sweepOnce(ctx)
		}
	}
}

func (r *HoldReaper) sweepOnce(ctx context.Context) {
	holds, err := r.Holds.ListExpiredHolds(ctx, r.Batch)
	if err != nil {
		r.Logger.WarnContext(ctx, "hold reaper list failed", "err", err)
		return
	}
	if len(holds) == 0 {
		return
	}
	r.Logger.InfoContext(ctx, "hold reaper sweeping", "count", len(holds))
	for _, h := range holds {
		if err := r.Billing.Release(ctx, h.RequestID); err != nil {
			r.Logger.WarnContext(ctx, "hold release failed",
				"err", err, "request_id", h.RequestID, "amount_cents", h.AmountCents)
			continue
		}
		// Best-effort: even if the secondary mark fails, billing.Release
		// has already updated wallet state — repeating it is idempotent.
		_ = r.Holds.MarkHoldExpired(ctx, h.RequestID)
		r.released.Add(1)
	}
}

// Released exposes the lifetime counter for /metrics.
func (r *HoldReaper) Released() int64 { return r.released.Load() }
