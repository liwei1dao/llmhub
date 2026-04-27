// Package gateway is the public HTTP edge: protocol translation,
// authentication, billing pre-authorization, scheduling, and upstream
// proxying. M3 shipped the skeleton with a mock provider; M4 wired the
// real provider dispatcher; M6 adds rate-limiting.
package gateway

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/llmhub/llmhub/internal/capability/chat"
	"github.com/llmhub/llmhub/internal/capability/embedding"
	"github.com/llmhub/llmhub/internal/platform/ratelimit"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// Deps is the gateway wiring — each field is a small interface so the
// package can be unit-tested with in-memory fakes.
type Deps = chat.Deps

// EmbeddingDeps wires the embedding capability handler. Allowing it
// to be optional keeps M3-era deployments without embedding running.
type EmbeddingDeps = embedding.Deps

// Options bundles non-core gateway knobs. Kept separate from Deps so
// test constructors don't have to care about operational tuning.
type Options struct {
	RateLimiter        ratelimit.Limiter // nil → no-op
	RateLimitPerSecond int               // 0 → effectively unlimited

	// Embedding is optional. When non-nil the gateway exposes
	// /v1/embeddings using these deps.
	Embedding *EmbeddingDeps
}

// NewServer returns the composed http.Handler.
func NewServer(_ *slog.Logger, deps Deps, opts Options) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(120 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// OpenAI-compatible surface, protected by per-key rate-limit.
	r.Route("/v1", func(r chi.Router) {
		if opts.RateLimiter != nil && opts.RateLimitPerSecond > 0 {
			r.Use(rateLimitMiddleware(opts.RateLimiter, opts.RateLimitPerSecond))
		}
		r.Method(http.MethodPost, "/chat/completions", chat.NewHandler(deps))
		if opts.Embedding != nil {
			r.Method(http.MethodPost, "/embeddings", embedding.NewHandler(*opts.Embedding))
		}
	})

	return r
}
