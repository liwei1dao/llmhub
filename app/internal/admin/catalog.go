package admin

import (
	"net/http"
	"strconv"

	"github.com/llmhub/llmhub/pkg/httpx"
)

func (s *Server) listPricing(w http.ResponseWriter, r *http.Request) {
	if s.catalog == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "catalog repo not wired")
		return
	}
	model := r.URL.Query().Get("model_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.catalog.ListAdminPricing(r.Context(), model, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": items})
}
