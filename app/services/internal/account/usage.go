package account

import (
	"net/http"
	"strconv"
	"time"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// handleUsageSeries serves GET /api/user/usage/series?range=7d
// Range options: 1d / 7d / 30d. Output is one bucket per (day, capability).
func (s *Server) handleUsageSeries(w http.ResponseWriter, r *http.Request) {
	if s.metering == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "metering repo not wired")
		return
	}
	uid := userIDFrom(r.Context())
	rangeStr := r.URL.Query().Get("range")
	days := parseRangeDays(rangeStr, 7)

	now := time.Now().UTC()
	to := now.Truncate(24 * time.Hour).Add(24 * time.Hour) // exclusive end-of-today
	from := to.AddDate(0, 0, -days)

	rows, err := s.metering.UserUsageSeries(r.Context(), uid, from, to)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, b := range rows {
		out = append(out, map[string]any{
			"day":               b.Day.Format("2006-01-02"),
			"capability_id":     b.CapabilityID,
			"calls":             b.Calls,
			"success_calls":     b.SuccessCalls,
			"tokens_in":         b.TokensIn,
			"tokens_out":        b.TokensOut,
			"audio_seconds":     b.AudioSeconds,
			"characters":        b.Characters,
			"cost_retail_cents": b.CostRetailCents,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"from": from.Format("2006-01-02"),
		"to":   to.Format("2006-01-02"),
		"data": out,
	})
}

// parseRangeDays accepts "1d" / "7d" / "30d" / bare integer strings.
// Falls back to fallback for unrecognized input.
func parseRangeDays(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	if len(s) > 1 && s[len(s)-1] == 'd' {
		s = s[:len(s)-1]
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 90 {
		return fallback
	}
	return n
}
