// Package llmhub is the Go client SDK for the LLMHub aggregation
// platform.
//
// The platform never proxies upstream calls. Instead this SDK:
//
//  1. Trades the user's api_key + a SKU id for a short-lived "lease"
//     containing the *real* upstream credential, via a single TLS
//     POST to /sdk/credentials/issue.
//  2. Calls the upstream provider (DeepSeek / 火山方舟 / ...) directly
//     with the leased credential.
//  3. Asynchronously reports the call outcome to /sdk/usage/report so
//     the platform can decrement the user's quota and adjust the
//     binding's health score.
//
// Security stance ("honest developer" tier): leases are kept only in
// process memory, zeroed on expiry, and never written to disk. A
// determined attacker with a debugger can still observe the leased
// upstream key — strong anti-reverse requires obfuscation tooling
// (e.g. garble + cgo) layered on top of this SDK.
package llmhub

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds the SDK's wiring. APIKey is the only required field;
// every other knob has a reasonable default.
type Config struct {
	// APIKey is the user's platform key, issued at signup and shown
	// once on the console. Required.
	APIKey string

	// BaseURL is the LLMHub platform endpoint (the SDK calls
	// /sdk/credentials/issue and /sdk/usage/report on it). When empty
	// the SDK uses DefaultBaseURL.
	BaseURL string

	// HTTPClient lets callers override the underlying http.Client
	// (for custom transports / cert pinning / corporate proxies).
	// When nil a Client with sensible timeouts is used.
	HTTPClient *http.Client

	// LeaseLeadTime is how far ahead of expiry the SDK proactively
	// refreshes a lease. Default 60s; clamp [10s, 5min].
	LeaseLeadTime time.Duration

	// ReportTimeout caps how long an async usage-report POST may
	// take. Default 10s. Failed reports are dropped (best-effort).
	ReportTimeout time.Duration

	// UserAgent is the HTTP UA string sent on every outbound call,
	// both to the platform and to upstream providers. Defaults to
	// "llmhub-go-sdk/<Version>".
	UserAgent string
}

// DefaultBaseURL is the public LLMHub platform endpoint. The console
// hands users a download whose api_base_url is already pre-baked into
// a config file shipped alongside the binary release; this default is
// the fallback for ad-hoc `go get` consumers.
const DefaultBaseURL = "https://api.llmhub.com"

// Version is bumped on every published release; surfaced in UA + the
// SDK's "/sdk/services" call so the platform can flag legacy clients.
const Version = "0.1.0"

// Client is the entry point. Create one per process; safe for
// concurrent use by multiple goroutines.
type Client struct {
	cfg     Config
	http    *http.Client
	leases  *leaseCache
	report  *reportSink
	closed  chan struct{}
	closeMu sync.Once
}

// New constructs a Client. Returns ErrMissingAPIKey if cfg.APIKey is
// empty.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrMissingAPIKey
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 120 * time.Second}
	}
	if cfg.LeaseLeadTime <= 0 {
		cfg.LeaseLeadTime = 60 * time.Second
	}
	if cfg.LeaseLeadTime < 10*time.Second {
		cfg.LeaseLeadTime = 10 * time.Second
	}
	if cfg.LeaseLeadTime > 5*time.Minute {
		cfg.LeaseLeadTime = 5 * time.Minute
	}
	if cfg.ReportTimeout <= 0 {
		cfg.ReportTimeout = 10 * time.Second
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "llmhub-go-sdk/" + Version
	}

	c := &Client{
		cfg:    cfg,
		http:   cfg.HTTPClient,
		closed: make(chan struct{}),
	}
	c.leases = newLeaseCache(c)
	c.report = newReportSink(c)
	return c, nil
}

// Close flushes outstanding usage reports and zeroes any cached lease
// material. Safe to call multiple times. Always defer Close at the
// top of main so leased upstream credentials don't outlive the
// process unnecessarily.
func (c *Client) Close() error {
	c.closeMu.Do(func() {
		close(c.closed)
		c.report.shutdown()
		c.leases.purgeAll()
	})
	return nil
}

// ----- public errors -----

// ErrMissingAPIKey is returned by New when Config.APIKey is empty.
var ErrMissingAPIKey = errors.New("llmhub: APIKey is required")

// APIError is returned when the platform or an upstream returns a
// non-2xx response. The Code/Message fields mirror the platform error
// envelope; Status is the HTTP status code (4xx for client problems,
// 5xx for server / upstream).
type APIError struct {
	Status  int
	Code    string
	Message string
	// Source is "platform" for /sdk/* errors, "upstream" for upstream
	// provider errors.
	Source string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %d %s: %s", e.Source, e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %d: %s", e.Source, e.Status, e.Message)
}

// IsRetryable returns true for transient platform / upstream errors
// (rate limits, timeouts, 5xx). Callers can use this to decide
// whether to retry with exponential backoff.
func (e *APIError) IsRetryable() bool {
	switch e.Status {
	case http.StatusTooManyRequests, http.StatusRequestTimeout, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return e.Status >= 500
}

// ctxOrClosed returns a context that fires when either the supplied
// ctx is cancelled OR the client is closed.
func (c *Client) ctxOrClosed(ctx context.Context) (context.Context, context.CancelFunc) {
	cctx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-c.closed:
			cancel()
		case <-cctx.Done():
		}
	}()
	return cctx, cancel
}
