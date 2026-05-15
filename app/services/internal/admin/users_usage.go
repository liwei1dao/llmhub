package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// getUserUsage handles GET /api/admin/users/{id}/usage?days=30.
//
// 把"用户用量"页要展示的所有汇总数据一次打包给前端，免得 admin 页
// 反复发请求：
//   - totals      —— [from, to) 窗口的合计 KPI
//   - by_sku      —— 按 SKU 拆分，排序按调用量降序
//   - by_status   —— success / 各类失败的分布
//   - daily       —— 每日总量（画 30d 趋势图）
//   - recent      —— 最近 N 条 call_logs
//
// days 可选 1..90，默认 30；limit_recent 可选 1..200，默认 50。
func (s *Server) getUserUsage(w http.ResponseWriter, r *http.Request) {
	if s.metering == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "metering repo not wired")
		return
	}
	uid, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || uid <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "user id must be a positive integer")
		return
	}

	q := r.URL.Query()
	days := parseAdminRangeDays(q.Get("days"), 30)
	limitRecent := parseAdminLimit(q.Get("limit_recent"), 50)

	now := time.Now().UTC()
	to := now.Truncate(24 * time.Hour).Add(24 * time.Hour) // exclusive end-of-today
	from := to.AddDate(0, 0, -days)

	totals, err := s.metering.UserUsageTotals(r.Context(), uid, from, to)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "usage totals: "+err.Error())
		return
	}
	bySKU, err := s.metering.UserUsageBySKU(r.Context(), uid, from, to)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "usage by sku: "+err.Error())
		return
	}
	byStatus, err := s.metering.UserUsageByStatus(r.Context(), uid, from, to)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "usage by status: "+err.Error())
		return
	}
	daily, err := s.metering.UserDailySeries(r.Context(), uid, from, to)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "usage daily: "+err.Error())
		return
	}
	recent, err := s.metering.UserRecentCalls(r.Context(), uid, limitRecent)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "recent calls: "+err.Error())
		return
	}

	skuRows := make([]map[string]any, 0, len(bySKU))
	for _, b := range bySKU {
		skuRows = append(skuRows, map[string]any{
			"sku_id":            b.SKUID,
			"vendor_id":         b.VendorID,
			"product_id":        b.ProductID,
			"calls":             b.Calls,
			"success_calls":     b.SuccessCalls,
			"tokens_in":         b.TokensIn,
			"tokens_out":        b.TokensOut,
			"cost_retail_cents": b.CostRetailCents,
			"last_used_at":      b.LastUsedAt,
		})
	}
	statusRows := make([]map[string]any, 0, len(byStatus))
	for _, b := range byStatus {
		statusRows = append(statusRows, map[string]any{
			"status": b.Status,
			"count":  b.Count,
		})
	}
	dailyRows := make([]map[string]any, 0, len(daily))
	for _, b := range daily {
		dailyRows = append(dailyRows, map[string]any{
			"day":               b.Day.Format("2006-01-02"),
			"calls":             b.Calls,
			"success_calls":     b.SuccessCalls,
			"tokens_in":         b.TokensIn,
			"tokens_out":        b.TokensOut,
			"cost_retail_cents": b.CostRetailCents,
		})
	}
	recentRows := make([]map[string]any, 0, len(recent))
	for _, c := range recent {
		recentRows = append(recentRows, map[string]any{
			"ts":          c.Timestamp,
			"request_id":  c.RequestID,
			"sku_id":      c.SKUID,
			"vendor_id":   c.VendorID,
			"product_id":  c.ProductID,
			"status":      c.Status,
			"error_code":  c.ErrorCode,
			"duration_ms": c.DurationMs,
			"ttfb_ms":     c.TTFBMs,
			"tokens_in":   c.TokensIn,
			"tokens_out":  c.TokensOut,
		})
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"range_days": days,
		"from":       from.Format("2006-01-02"),
		"to":         to.Format("2006-01-02"),
		"totals": map[string]any{
			"calls":             totals.Calls,
			"success_calls":     totals.SuccessCalls,
			"tokens_in":         totals.TokensIn,
			"tokens_out":        totals.TokensOut,
			"cost_retail_cents": totals.CostRetailCents,
			"unique_skus":       totals.UniqueSKUs,
			"avg_latency_ms":    totals.AvgLatencyMs,
			"avg_ttfb_ms":       totals.AvgTTFBMs,
		},
		"by_sku":    skuRows,
		"by_status": statusRows,
		"daily":     dailyRows,
		"recent":    recentRows,
	})
}

// parseAdminRangeDays accepts "1d" / "7d" / "30d" / bare integer strings.
// Falls back when input is missing or out of [1, 90].
func parseAdminRangeDays(s string, fallback int) int {
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

func parseAdminLimit(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 200 {
		return fallback
	}
	return n
}
