package worker

import "context"

// SweepForTest exposes sweepOnce so tests can drive the reaper without
// the ticker / goroutine of Run.
func SweepForTest(r *HoldReaper, ctx context.Context) { r.sweepOnce(ctx) }
