package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/pkg/httpx"
)

// confirmRechargeReq carries the channel-side proof of payment that
// admins paste in when manually confirming a recharge order. For real
// PSPs this endpoint is replaced by a signed webhook handler.
type confirmRechargeReq struct {
	ChannelOrderID string `json:"channel_order_id"`
}

// handleConfirmRecharge is POST /api/admin/recharges/{order_no}/confirm.
// Idempotent: re-confirming a paid order is a no-op.
func (s *Server) handleConfirmRecharge(w http.ResponseWriter, r *http.Request) {
	if s.wallet == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "wallet service not wired")
		return
	}
	orderNo := chi.URLParam(r, "order_no")
	if orderNo == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "order_no required")
		return
	}
	var req confirmRechargeReq
	_ = httpx.DecodeJSON(w, r, &req)
	if err := s.wallet.ConfirmRecharge(r.Context(), orderNo, req.ChannelOrderID); err != nil {
		s.logger.ErrorContext(r.Context(), "admin confirm recharge failed", "err", err, "order_no", orderNo)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "paid"})
}
