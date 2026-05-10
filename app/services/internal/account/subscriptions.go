package account

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// handleListSubscriptions returns every active subscription that the
// authenticated user holds, hydrated with SKU metadata so the user
// console can render the 服务订阅 page in one round-trip.
//
// Mirrors GET /sdk/services (which is the SDK's discovery endpoint),
// but lives under /api/user so the web console can use session auth
// instead of the SDK bearer.
func (s *Server) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "subscriptions repo not wired")
		return
	}
	uid := userIDFrom(r.Context())
	rows, err := s.subs.ListByUser(r.Context(), uid)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "user list subscriptions", "err", err, "user_id", uid)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, sub := range rows {
		row := map[string]any{
			"id":              sub.ID,
			"sku_id":          sub.SKUID,
			"plan_kind":       sub.PlanKind,
			"plan_name":       sub.PlanName,
			"quota_total":     sub.QuotaTotal,
			"quota_used":      sub.QuotaUsed,
			"quota_remaining": sub.QuotaTotal - sub.QuotaUsed,
			"period_start":    sub.PeriodStart,
			"period_end":      sub.PeriodEnd,
			"qps_limit":       sub.QPSLimit,
			"daily_used":      sub.DailyUsed,
			"daily_used_date": sub.DailyUsedDate,
			"auto_renew":      sub.AutoRenew,
		}
		if sub.DailyQuotaLimit != nil {
			row["daily_quota_limit"] = *sub.DailyQuotaLimit
		}
		// Hydrate SKU metadata so the UI can render display_name + capability
		// without a second request.
		if s.catalog != nil {
			if sku, err := s.catalog.LookupSKU(r.Context(), sub.SKUID); err == nil && sku != nil {
				row["display_name"] = sku.DisplayName
				row["category_id"] = sku.CategoryID
				row["billing_unit"] = sku.BillingUnit
				row["capability"] = sku.Capability
				row["upstream_model"] = sku.UpstreamModel
				if sku.InputCents != nil {
					row["input_per_unit_cents"] = *sku.InputCents
				}
				if sku.OutputCents != nil {
					row["output_per_unit_cents"] = *sku.OutputCents
				}
			}
		}
		out = append(out, row)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// handleRevokeAPIKey flips a key to status='revoked'. The cross-check
// in iam.RevokeAPIKey makes sure callers can only revoke their own keys.
func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be a positive integer")
		return
	}
	if err := s.iam.RevokeAPIKey(r.Context(), uid, id); err != nil {
		// iam.RevokeAPIKey returns ErrNotFound when the key doesn't exist
		// or doesn't belong to this user — both cases surface as 404 to
		// avoid leaking key-id existence across accounts.
		if errors.Is(err, iamrepo.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "api key not found")
			return
		}
		s.logger.ErrorContext(r.Context(), "user revoke api key", "err", err, "user_id", uid, "key_id", id)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
