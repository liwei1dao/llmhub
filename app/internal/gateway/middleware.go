package gateway

import (
	"net/http"
	"strings"

	"github.com/llmhub/llmhub/internal/platform/ratelimit"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// rateLimitMiddleware enforces per-API-key QPS using the provided limiter.
// Unauthenticated requests pass through — the chat handler later rejects
// them with a structured 401, which keeps the middleware scope tight.
//
// Keyed off the Authorization header so an attacker flooding with many
// user accounts can't exhaust a shared budget.
func rateLimitMiddleware(lim ratelimit.Limiter, perSecond int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := bearerKey(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			ok, err := lim.Allow(r.Context(), key, perSecond)
			if err != nil {
				// Limiter backend errored — fail open.
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "per-key rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerKey(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}
