package admin

import (
	"net/http"
	"time"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// dashboardStats handles GET /api/admin/dashboard/stats. Returns a few
// counters the operator wants on the home page without having to drill
// into individual pages.
//
// Counters are computed via single SQL queries — fine at v0.2's data
// volumes; if pool.leases ever grows past O(millions) we'll move to
// materialized snapshots refreshed by a worker.
func (s *Server) dashboardStats(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil || s.iam == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool / iam not wired")
		return
	}
	pp := s.pool.Repo().Pool()
	ctx := r.Context()

	out := map[string]any{
		"server_time": time.Now().UTC().Format(time.RFC3339),
	}

	// 活 lease + 累计签发
	var leasesActive, leasesTotal int64
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM pool.leases WHERE status='active' AND expires_at > NOW()`).Scan(&leasesActive); err != nil {
		s.logger.WarnContext(ctx, "dashboard: leases active", "err", err)
	}
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM pool.leases`).Scan(&leasesTotal); err != nil {
		s.logger.WarnContext(ctx, "dashboard: leases total", "err", err)
	}
	out["leases_active"] = leasesActive
	out["leases_total"] = leasesTotal

	// 活订阅 + 总订阅
	var subsActive, subsTotal int64
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM iam.subscriptions WHERE status='active'`).Scan(&subsActive); err != nil {
		s.logger.WarnContext(ctx, "dashboard: subs active", "err", err)
	}
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM iam.subscriptions`).Scan(&subsTotal); err != nil {
		s.logger.WarnContext(ctx, "dashboard: subs total", "err", err)
	}
	out["subscriptions_active"] = subsActive
	out["subscriptions_total"] = subsTotal

	// 用户 + 活 API key
	var usersTotal, apiKeysActive int64
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM iam.users`).Scan(&usersTotal); err != nil {
		s.logger.WarnContext(ctx, "dashboard: users", "err", err)
	}
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM iam.api_keys WHERE status='active'`).Scan(&apiKeysActive); err != nil {
		s.logger.WarnContext(ctx, "dashboard: api keys", "err", err)
	}
	out["users_total"] = usersTotal
	out["api_keys_active"] = apiKeysActive

	// 凭据池
	var credsActive, credsCooldown int64
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM pool.credentials WHERE status='active'`).Scan(&credsActive); err != nil {
		s.logger.WarnContext(ctx, "dashboard: creds active", "err", err)
	}
	if err := pp.QueryRow(ctx, `SELECT COUNT(*) FROM pool.credentials WHERE status='cooldown'`).Scan(&credsCooldown); err != nil {
		s.logger.WarnContext(ctx, "dashboard: creds cooldown", "err", err)
	}
	out["credentials_active"] = credsActive
	out["credentials_cooldown"] = credsCooldown

	// 今日调用量（按 metering.call_logs.ts >= today UTC 起始）
	var callsToday, callsSuccessToday int64
	if err := pp.QueryRow(ctx,
		`SELECT COUNT(*) FROM metering.call_logs WHERE ts >= date_trunc('day', NOW())`,
	).Scan(&callsToday); err != nil {
		s.logger.WarnContext(ctx, "dashboard: calls today", "err", err)
	}
	if err := pp.QueryRow(ctx,
		`SELECT COUNT(*) FROM metering.call_logs WHERE ts >= date_trunc('day', NOW()) AND status='success'`,
	).Scan(&callsSuccessToday); err != nil {
		s.logger.WarnContext(ctx, "dashboard: calls success today", "err", err)
	}
	out["calls_today"] = callsToday
	out["calls_success_today"] = callsSuccessToday

	// 待确认充值
	var rechargesPending int64
	if err := pp.QueryRow(ctx,
		`SELECT COUNT(*) FROM wallet.recharges WHERE status='pending'`,
	).Scan(&rechargesPending); err != nil {
		s.logger.WarnContext(ctx, "dashboard: recharges pending", "err", err)
	}
	out["recharges_pending"] = rechargesPending

	httpx.JSON(w, http.StatusOK, out)
}
