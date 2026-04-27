package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// listRecon is GET /api/admin/reconciliation?day=YYYY-MM-DD&provider=...
func (s *Server) listRecon(w http.ResponseWriter, r *http.Request) {
	if s.metering == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "metering repo not wired")
		return
	}
	var day time.Time
	if v := r.URL.Query().Get("day"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "day must be YYYY-MM-DD")
			return
		}
		day = t
	}
	provider := r.URL.Query().Get("provider")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.metering.ListRecon(r.Context(), day, provider, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": rows})
}
