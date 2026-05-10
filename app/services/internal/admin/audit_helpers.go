package admin

import (
	"net/http"
	"strconv"

	"github.com/llmhub/llmhub/internal/audit"
)

// recordAdmin is a thin shim that fills in the actor + IP/UA fields
// from the http.Request, so handler code only has to specify the
// action / target / result / payload bits that change.
//
// In v0.2 we don't have per-admin login (everyone shares X-Admin-Token),
// so actor_id is always 0; ActorType is hard-coded "admin" because
// that's the only path mounted under /api/admin/*.
func (s *Server) recordAdmin(r *http.Request, action, targetType, targetID, result string, payload map[string]any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(r.Context(), audit.Event{
		ActorType:  "admin",
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Result:     result,
		Payload:    payload,
	})
}

// idStr is shorthand for strconv.FormatInt — handlers use it to
// stringify int64 ids for audit target_id without verbose imports.
func idStr(n int64) string { return strconv.FormatInt(n, 10) }
