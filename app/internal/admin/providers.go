package admin

import (
	"net/http"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// listProviders is GET /api/admin/providers.
func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	if s.catalog == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "catalog repo not wired")
		return
	}
	rows, err := s.catalog.ListProviders(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": rows})
}
