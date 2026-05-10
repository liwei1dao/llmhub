package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/internal/wallet"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// getUserWallet handles GET /api/admin/users/{id}/wallet.
//
// Returns the user's CNY wallet snapshot, recent recharges, and
// 30-day spend aggregated from metering.call_logs.cost_retail_cents.
// Used by the admin user-detail page to surface 余额/消费 without
// having to assemble three separate calls on the client.
func (s *Server) getUserWallet(w http.ResponseWriter, r *http.Request) {
	if s.wallet == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "wallet service not wired")
		return
	}
	uid, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || uid <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "user id must be a positive integer")
		return
	}

	out := map[string]any{}

	// Balance snapshot. Account may not exist yet (user never recharged) — that's not an error.
	acc, err := s.wallet.GetAccount(r.Context(), uid)
	switch {
	case errors.Is(err, wallet.ErrAccountNotFound):
		out["account_exists"] = false
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	default:
		out["account_exists"] = true
		out["balance_cents"] = acc.BalanceCents
		out["frozen_cents"] = acc.FrozenCents
		out["currency"] = acc.Currency
		out["total_recharged_cents"] = acc.TotalRechargedCents
		out["total_spent_cents"] = acc.TotalSpentCents
	}

	// Recent recharges.
	if rs, err := s.wallet.ListRecharges(r.Context(), uid, 10); err != nil {
		s.logger.WarnContext(r.Context(), "admin user wallet: list recharges", "err", err, "user_id", uid)
		out["recharges"] = []any{}
	} else {
		recharges := make([]map[string]any, 0, len(rs))
		for _, rg := range rs {
			recharges = append(recharges, map[string]any{
				"order_no":         rg.OrderNo,
				"amount_cents":     rg.AmountCents,
				"channel":          rg.Channel,
				"status":           rg.Status,
				"paid_at":          rg.PaidAt,
				"created_at":       rg.CreatedAt,
				"channel_order_id": rg.ChannelOrderID,
			})
		}
		out["recharges"] = recharges
	}

	// 30-day retail spend, aggregated from call logs.
	// Falls back to 0 if pool not wired or the table is empty.
	if s.pool != nil {
		var spent30dCents int64
		var calls30d int64
		err := s.pool.Repo().Pool().QueryRow(r.Context(),
			`SELECT COALESCE(SUM(cost_retail_cents), 0)::BIGINT, COUNT(*)
			 FROM metering.call_logs
			 WHERE user_id = $1 AND ts >= NOW() - INTERVAL '30 days'`,
			uid,
		).Scan(&spent30dCents, &calls30d)
		if err != nil {
			s.logger.WarnContext(r.Context(), "admin user wallet: 30d spend", "err", err, "user_id", uid)
		}
		out["spent_30d_cents"] = spent30dCents
		out["calls_30d"] = calls30d
	}

	httpx.JSON(w, http.StatusOK, out)
}
