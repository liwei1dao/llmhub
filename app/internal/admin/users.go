package admin

import (
	"net/http"
	"strconv"

	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// ListUsers is the admin GET /api/admin/users handler.
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if s.iam == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "iam repo not wired")
		return
	}
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	users, err := s.iam.ListAdminUsers(r.Context(), q, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": coerceUsers(users)})
}

// coerceUsers flattens []AdminUser to a display-friendly slice.
func coerceUsers(in []iamrepo.AdminUser) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, u := range in {
		out = append(out, map[string]any{
			"id":                       u.ID,
			"email":                    u.Email,
			"phone":                    u.Phone,
			"status":                   u.Status,
			"risk_score":               u.RiskScore,
			"qps_limit":                u.QPSLimit,
			"daily_spend_limit_cents":  u.DailySpendLimitCents,
			"created_at":               u.CreatedAt,
			"last_login_at":            u.LastLoginAt,
		})
	}
	return out
}
