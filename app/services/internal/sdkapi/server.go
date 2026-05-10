// Package sdkapi exposes the platform's SDK-facing HTTP surface. Unlike
// the (now-deprecated) /v1/chat/completions gateway, this package never
// proxies upstream calls — it only:
//
//   1. Verifies the SDK's (id, key) and the user's SKU subscription
//   2. Picks a healthy upstream binding under the SKU's product/capability
//   3. Resolves the binding's auth_payload from vault
//   4. Mints a short-lived lease and hands the *real* upstream credentials
//      back to the SDK (never to user code — the SDK is the security
//      boundary)
//   5. Accepts asynchronous usage reports and decrements user quota
//
// The SDK then talks to the upstream provider directly, with the lease's
// auth_payload, and reports back via /sdk/usage/report. The platform is
// not on the critical request path.
package sdkapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/internal/catalog"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/platform/vault"
	"github.com/llmhub/llmhub/internal/pool"
)

// Deps wires the SDK-API handler to the services it needs. We deliberately
// keep this minimal: the SDK API is a thin orchestration layer.
type Deps struct {
	Logger     *slog.Logger
	Auth       AuthResolver           // verifies (id, key) → user
	Catalog    *catalog.Service       // resolves SKU
	Pool       *pool.Service          // pool.PickBinding + repo.Leases()
	Subs       *iamrepo.SubscriptionRepo
	Metering   *meteringrepo.Repo     // appends call_logs from usage reports
	Vault      vault.Resolver         // resolves auth_payload_ref
	LeaseTTLSec int                   // 0 → 900 (15 min)
}

// AuthResolver is the auth surface the SDK API uses. The id is the
// public api_key prefix-or-id; the key is the secret. The handler maps
// "<id>:<key>" or just "<key>" Bearer values to a user.
type AuthResolver interface {
	AuthenticateAPIKey(ctx context.Context, plaintext string) (userID, apiKeyID int64, err error)
}

// Mount attaches /sdk/* routes onto the supplied router. Suggested
// composition:
//
//	r := chi.NewRouter()
//	sdkapi.New(deps).Mount(r)
//	root.Mount("/", r)
//
// or wrap into the account-server router so /sdk shares its middleware.
type Server struct{ d Deps }

// New constructs a Server. Defaults are filled in.
func New(d Deps) *Server {
	if d.LeaseTTLSec == 0 {
		d.LeaseTTLSec = 15 * 60
	}
	return &Server{d: d}
}

// Mount registers /sdk/* under the supplied chi router.
func (s *Server) Mount(r chi.Router) {
	r.Route("/sdk", func(r chi.Router) {
		r.Post("/credentials/issue", s.handleIssue)
		r.Post("/usage/report", s.handleUsageReport)
		r.Get("/services", s.handleListServices) // SDK boot-time discovery
	})
}

// handleListServices returns the SKUs the authenticated user is subscribed
// to. SDK calls this on boot to know which models it can be invoked with.
func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	auth, err := s.authenticate(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	subs, err := s.d.Subs.ListByUser(r.Context(), auth.UserID)
	if err != nil {
		s.d.Logger.ErrorContext(r.Context(), "list subscriptions", "err", err, "user_id", auth.UserID)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list subscriptions")
		return
	}
	out := make([]map[string]any, 0, len(subs))
	for _, sub := range subs {
		row := map[string]any{
			"sku_id":          sub.SKUID,
			"plan_kind":       sub.PlanKind,
			"plan_name":       sub.PlanName,
			"quota_total":     sub.QuotaTotal,
			"quota_used":      sub.QuotaUsed,
			"quota_remaining": sub.QuotaTotal - sub.QuotaUsed,
			"period_end":      sub.PeriodEnd,
			"qps_limit":       sub.QPSLimit,
		}
		// Hydrate with SKU metadata for client convenience.
		if sku, err := s.d.Catalog.LookupSKU(r.Context(), sub.SKUID); err == nil && sku != nil {
			row["display_name"] = sku.DisplayName
			row["category_id"] = sku.CategoryID
			row["billing_unit"] = sku.BillingUnit
			row["capability"] = sku.Capability
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}
