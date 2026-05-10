package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/llmhub/llmhub/internal/audit"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// auditList returns the most-recent audit rows. Filters: actor_id /
// action / target_type / target_id; default limit 200.
func (s *Server) auditList(w http.ResponseWriter, r *http.Request) {
	pg, ok := s.audit.(*audit.PgRecorder)
	if !ok {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "audit recorder not wired")
		return
	}
	q := r.URL.Query()
	actorID, _ := strconv.ParseInt(q.Get("actor_id"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	rows, err := pg.List(r.Context(), audit.Filter{
		ActorID:    actorID,
		Action:     q.Get("action"),
		TargetType: q.Get("target_type"),
		TargetID:   q.Get("target_id"),
		Limit:      limit,
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin list audit", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, rec := range rows {
		row := map[string]any{
			"id":          rec.ID,
			"actor_type":  rec.ActorType,
			"actor_id":    rec.ActorID,
			"action":      rec.Action,
			"target_type": rec.TargetType,
			"target_id":   rec.TargetID,
			"ip":          rec.IP,
			"user_agent":  rec.UserAgent,
			"result":      rec.Result,
			"created_at":  rec.CreatedAt,
		}
		// Decode payload JSON so the UI can render structured fields
		// without doing a second JSON.parse on the client.
		if len(rec.Payload) > 0 {
			var p any
			if err := json.Unmarshal(rec.Payload, &p); err == nil {
				row["payload"] = p
			}
		}
		out = append(out, row)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}
