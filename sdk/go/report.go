package llmhub

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// usageReport is one event the SDK sends to /sdk/usage/report.
type usageReport struct {
	LeaseID     string `json:"lease_id"`
	RequestID   string `json:"request_id,omitempty"`
	InputUnits  int64  `json:"input_units,omitempty"`
	OutputUnits int64  `json:"output_units,omitempty"`
	Status      string `json:"status"`
	ErrorCode   string `json:"error_code,omitempty"`
	LatencyMs   int64  `json:"latency_ms,omitempty"`
	TTFBMs      int64  `json:"ttfb_ms,omitempty"`
}

// reportSink owns a buffered channel + worker pool that flushes usage
// events back to the platform. Sends are best-effort: when the buffer
// is full we drop on the floor instead of blocking the caller's
// chat / embedding / ... call.
//
// The size + concurrency are deliberately small — usage reports are
// short, and a busy SDK process generating thousands of reports per
// second is more likely a runaway loop than legitimate traffic.
type reportSink struct {
	c       *Client
	ch      chan usageReport
	workers sync.WaitGroup
	stopped chan struct{}
	once    sync.Once
}

func newReportSink(c *Client) *reportSink {
	r := &reportSink{
		c:       c,
		ch:      make(chan usageReport, 256),
		stopped: make(chan struct{}),
	}
	const workerCount = 2
	for i := 0; i < workerCount; i++ {
		r.workers.Add(1)
		go r.run()
	}
	return r
}

// enqueue is non-blocking. Returns true if accepted, false if dropped.
func (r *reportSink) enqueue(ev usageReport) bool {
	select {
	case r.ch <- ev:
		return true
	default:
		// Buffer full — drop. The platform tolerates missing reports
		// (best-effort by design); persistent drops show up in admin
		// anomaly alerts.
		return false
	}
}

func (r *reportSink) shutdown() {
	r.once.Do(func() {
		close(r.ch)
		// Bound the wait so a misbehaving network doesn't make Close
		// hang indefinitely; outstanding events get dropped.
		done := make(chan struct{})
		go func() { r.workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		close(r.stopped)
	})
}

func (r *reportSink) run() {
	defer r.workers.Done()
	for ev := range r.ch {
		r.send(ev)
	}
}

func (r *reportSink) send(ev usageReport) {
	ctx, cancel := context.WithTimeout(context.Background(), r.c.cfg.ReportTimeout)
	defer cancel()
	body, _ := json.Marshal(ev)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.c.cfg.BaseURL+"/sdk/usage/report", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.c.cfg.APIKey)
	req.Header.Set("User-Agent", r.c.cfg.UserAgent)
	resp, err := r.c.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = readAndDiscard(resp.Body)
}
