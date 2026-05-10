package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// SubscriptionView is the JSON shape exposed to admin web. We hydrate
// SKU display_name where possible (cheap — catalog cache hit).
type SubscriptionView struct {
	ID              int64   `json:"id"`
	UserID          int64   `json:"user_id"`
	SKUID           string  `json:"sku_id"`
	SKUName         string  `json:"sku_name,omitempty"`
	PlanKind        string  `json:"plan_kind"`
	PlanName        string  `json:"plan_name,omitempty"`
	QuotaTotal      int64   `json:"quota_total"`
	QuotaUsed       int64   `json:"quota_used"`
	QuotaRemaining  int64   `json:"quota_remaining"`
	PeriodStart     string  `json:"period_start"`
	PeriodEnd       string  `json:"period_end"`
	AutoRenew       bool    `json:"auto_renew"`
	Status          string  `json:"status"`
	QPSLimit        int32   `json:"qps_limit"`
	DailyQuotaLimit *int64  `json:"daily_quota_limit,omitempty"`
	DailyUsed       int64   `json:"daily_used"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func (s *Server) toSubscriptionView(ctx interface{ Done() <-chan struct{} }, sub *iamrepo.Subscription) SubscriptionView {
	v := SubscriptionView{
		ID:              sub.ID,
		UserID:          sub.UserID,
		SKUID:           sub.SKUID,
		PlanKind:        sub.PlanKind,
		PlanName:        sub.PlanName,
		QuotaTotal:      sub.QuotaTotal,
		QuotaUsed:       sub.QuotaUsed,
		QuotaRemaining:  sub.QuotaTotal - sub.QuotaUsed,
		PeriodStart:     sub.PeriodStart.Format(time.RFC3339),
		PeriodEnd:       sub.PeriodEnd.Format(time.RFC3339),
		AutoRenew:       sub.AutoRenew,
		Status:          sub.Status,
		QPSLimit:        sub.QPSLimit,
		DailyQuotaLimit: sub.DailyQuotaLimit,
		DailyUsed:       sub.DailyUsed,
		CreatedAt:       sub.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       sub.UpdatedAt.Format(time.RFC3339),
	}
	return v
}

// ───────────────────────── grant ─────────────────────────

type grantSubscriptionReq struct {
	SKUID           string  `json:"sku_id"`
	PlanKind        string  `json:"plan_kind"` // monthly / prepaid / trial
	PlanName        string  `json:"plan_name,omitempty"`
	QuotaTotal      int64   `json:"quota_total"`
	PeriodEndAt     *string `json:"period_end,omitempty"` // RFC3339; if nil, defaults by plan_kind
	AutoRenew       bool    `json:"auto_renew,omitempty"`
	QPSLimit        int32   `json:"qps_limit,omitempty"`
	DailyQuotaLimit *int64  `json:"daily_quota_limit,omitempty"`
}

// grantSubscription creates a new subscription for a user.
//
// POST /api/admin/users/{user_id}/subscriptions
//
// 默认周期：monthly = 30 天，prepaid = 90 天，trial = 7 天。可被
// period_end 覆盖。重复给同一个 (user, sku) 开订阅会被 unique 索引
// uq_subscriptions_user_sku_active 拒绝（409），admin 应该先 cancel
// 旧的再开新的。
func (s *Server) grantSubscription(w http.ResponseWriter, r *http.Request) {
	if s.iam == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "iam repo not wired")
		return
	}
	uid, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || uid <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "user id must be a positive integer")
		return
	}
	var req grantSubscriptionReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.SKUID == "" || req.PlanKind == "" || req.QuotaTotal <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"sku_id, plan_kind, quota_total are required")
		return
	}
	switch req.PlanKind {
	case "monthly", "prepaid", "trial":
	default:
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"plan_kind must be one of monthly / prepaid / trial")
		return
	}

	now := time.Now()
	periodEnd := now.AddDate(0, 1, 0) // monthly default
	if req.PlanKind == "prepaid" {
		periodEnd = now.AddDate(0, 3, 0)
	} else if req.PlanKind == "trial" {
		periodEnd = now.AddDate(0, 0, 7)
	}
	if req.PeriodEndAt != nil {
		t, err := time.Parse(time.RFC3339, *req.PeriodEndAt)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request",
				"period_end must be RFC3339 timestamp")
			return
		}
		if !t.After(now) {
			httpx.Error(w, http.StatusBadRequest, "invalid_request",
				"period_end must be in the future")
			return
		}
		periodEnd = t
	}

	sub := &iamrepo.Subscription{
		UserID:          uid,
		SKUID:           req.SKUID,
		PlanKind:        req.PlanKind,
		PlanName:        req.PlanName,
		QuotaTotal:      req.QuotaTotal,
		PeriodStart:     now,
		PeriodEnd:       periodEnd,
		AutoRenew:       req.AutoRenew,
		QPSLimit:        req.QPSLimit,
		DailyQuotaLimit: req.DailyQuotaLimit,
	}
	if _, err := s.iam.Subscriptions().Create(r.Context(), sub); err != nil {
		// Postgres unique-violation surfaces as a generic error here; we
		// return 409 so the UI can prompt the operator to cancel the
		// existing active subscription first.
		s.logger.WarnContext(r.Context(), "admin grant subscription failed",
			"err", err, "user_id", uid, "sku_id", req.SKUID)
		s.recordAdmin(r, "grant_subscription", "user", idStr(uid), "error", map[string]any{
			"sku_id": req.SKUID, "plan_kind": req.PlanKind, "error": err.Error(),
		})
		httpx.Error(w, http.StatusConflict, "subscription_conflict", err.Error())
		return
	}
	s.recordAdmin(r, "grant_subscription", "subscription", idStr(sub.ID), "ok", map[string]any{
		"user_id": uid, "sku_id": req.SKUID, "plan_kind": req.PlanKind,
		"quota_total": req.QuotaTotal,
	})
	httpx.JSON(w, http.StatusCreated, s.toSubscriptionView(r.Context(), sub))
}

// listUserSubscriptions handles GET /api/admin/users/{id}/subscriptions.
func (s *Server) listUserSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.iam == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "iam repo not wired")
		return
	}
	uid, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || uid <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "user id must be a positive integer")
		return
	}
	rows, err := s.iam.Subscriptions().ListByUser(r.Context(), uid)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin list user subscriptions",
			"err", err, "user_id", uid)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]SubscriptionView, 0, len(rows))
	for i := range rows {
		out = append(out, s.toSubscriptionView(r.Context(), &rows[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// ───────────────────────── patch / cancel ─────────────────────────

type patchSubscriptionReq struct {
	QuotaTotal      *int64  `json:"quota_total,omitempty"`
	PeriodEnd       *string `json:"period_end,omitempty"` // RFC3339
	AutoRenew       *bool   `json:"auto_renew,omitempty"`
	QPSLimit        *int32  `json:"qps_limit,omitempty"`
	DailyQuotaLimit *int64  `json:"daily_quota_limit,omitempty"`
	Status          *string `json:"status,omitempty"` // active / suspended / cancelled
	PlanName        *string `json:"plan_name,omitempty"`
}

// patchSubscription handles PATCH /api/admin/subscriptions/{id}. Used
// for: bumping quota mid-cycle, extending period, suspending, etc.
func (s *Server) patchSubscription(w http.ResponseWriter, r *http.Request) {
	if s.iam == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "iam repo not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "subscription id must be a positive integer")
		return
	}
	var req patchSubscriptionReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	patch := iamrepo.SubscriptionPatch{
		QuotaTotal:      req.QuotaTotal,
		AutoRenew:       req.AutoRenew,
		QPSLimit:        req.QPSLimit,
		DailyQuotaLimit: req.DailyQuotaLimit,
		Status:          req.Status,
		PlanName:        req.PlanName,
	}
	if req.PeriodEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.PeriodEnd)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request",
				"period_end must be RFC3339 timestamp")
			return
		}
		patch.PeriodEnd = &t
	}
	if patch.Status != nil {
		switch *patch.Status {
		case "active", "suspended", "cancelled", "expired":
		default:
			httpx.Error(w, http.StatusBadRequest, "invalid_request",
				"status must be one of active / suspended / cancelled / expired")
			return
		}
	}
	if err := s.iam.Subscriptions().Patch(r.Context(), id, patch); err != nil {
		if errors.Is(err, iamrepo.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.recordAdmin(r, "patch_subscription", "subscription", idStr(id), "ok", map[string]any{
		"quota_total":       patch.QuotaTotal,
		"period_end":        patch.PeriodEnd,
		"status":            patch.Status,
		"qps_limit":         patch.QPSLimit,
		"daily_quota_limit": patch.DailyQuotaLimit,
	})
	sub, err := s.iam.Subscriptions().Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, s.toSubscriptionView(r.Context(), sub))
}

// cancelSubscription handles DELETE /api/admin/subscriptions/{id}.
// Sets status='cancelled' on the row; idempotent — already-cancelled
// rows return 404 so the UI doesn't show stale "cancel" buttons.
func (s *Server) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	if s.iam == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "iam repo not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "subscription id must be a positive integer")
		return
	}
	if err := s.iam.Subscriptions().Cancel(r.Context(), id); err != nil {
		if errors.Is(err, iamrepo.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "subscription not found or already inactive")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.recordAdmin(r, "cancel_subscription", "subscription", idStr(id), "ok", nil)
	w.WriteHeader(http.StatusNoContent)
}
