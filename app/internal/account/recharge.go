package account

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/internal/wallet"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// ----- user-facing -----

type createRechargeReq struct {
	AmountCents int64  `json:"amount_cents"`
	Channel     string `json:"channel"`
}

func (s *Server) handleCreateRecharge(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	var req createRechargeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.AmountCents <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "amount_cents must be positive")
		return
	}
	if req.Channel == "" {
		req.Channel = "manual"
	}
	rg, err := s.wallet.CreateRecharge(r.Context(), wallet.CreateRechargeRequest{
		UserID:      uid,
		AmountCents: req.AmountCents,
		Channel:     req.Channel,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	// In production the response also carries the channel-specific
	// payment URL. For now the handler returns the order metadata so
	// the admin can confirm it manually.
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"order_no":     rg.OrderNo,
		"amount_cents": rg.AmountCents,
		"channel":      rg.Channel,
		"status":       rg.Status,
		"created_at":   rg.CreatedAt,
	})
}

func (s *Server) handleListRecharges(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.wallet.ListRecharges(r.Context(), uid, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, rg := range rows {
		out = append(out, map[string]any{
			"order_no":     rg.OrderNo,
			"amount_cents": rg.AmountCents,
			"channel":      rg.Channel,
			"status":       rg.Status,
			"paid_at":      rg.PaidAt,
			"created_at":   rg.CreatedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) handleGetRecharge(w http.ResponseWriter, r *http.Request) {
	orderNo := chi.URLParam(r, "order_no")
	if orderNo == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "order_no required")
		return
	}
	rg, err := s.wallet.GetRecharge(r.Context(), orderNo)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	uid := userIDFrom(r.Context())
	if rg.UserID != uid {
		httpx.Error(w, http.StatusNotFound, "not_found", "recharge not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"order_no":     rg.OrderNo,
		"amount_cents": rg.AmountCents,
		"channel":      rg.Channel,
		"status":       rg.Status,
		"paid_at":      rg.PaidAt,
		"created_at":   rg.CreatedAt,
	})
}
