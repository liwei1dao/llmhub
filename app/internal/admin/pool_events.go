package admin

import (
	"net/http"
	"strconv"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// listAccountEvents is GET /api/admin/pool/accounts/{id}/events.
func (s *Server) listAccountEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.repo.ListAccountEvents(r.Context(), id, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": rows})
}
