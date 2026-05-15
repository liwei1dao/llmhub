package account

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// handleActivateService 是用户自助开通服务的入口。
//
// 产品定位：开通仅代表"用户被授予调用该 SKU 的权限"，不收钱、不锁配额。
// 真正的扣费在 SDK 调用时按 catalog.platform_pricing 从钱包里走。
//
// 因此这里的策略是：
//   - 校验 SKU is_public=true AND status='active'（admin 必须先上架）
//   - 同一用户对同一 SKU 已有 active 订阅 → 直接 200 返回那条（幂等）
//   - 否则插入 plan_kind=monthly / quota_total=MaxInt64 / period=100 年
//     的"占位订阅" — 配额检查依然在 /sdk/credentials/issue 里跑，但
//     设这么大相当于没门槛
//
// 这跟 admin 的 grantSubscription 区别在于：
//   - 不要 plan_kind / quota_total / period 等运营参数
//   - 不需要 admin 鉴权（走 requireUser session）
//   - 重复开通幂等返回，而不是 409
//
// 路由：POST /api/user/subscriptions/activate {sku_id}
func (s *Server) handleActivateService(w http.ResponseWriter, r *http.Request) {
	if s.subs == nil || s.catalog == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error",
			"subscriptions / catalog not wired")
		return
	}

	var req struct {
		SKUID string `json:"sku_id"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.SKUID = strings.TrimSpace(req.SKUID)
	if req.SKUID == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "sku_id is required")
		return
	}

	// SKU 必须既上架（status=active）又对用户可见（is_public=true）。
	// 这是 admin 控制服务可见性的唯一门 —— 用户拿不到一个隐藏的 SKU。
	sku, err := s.catalog.LookupSKU(r.Context(), req.SKUID)
	if err != nil || sku == nil {
		httpx.Error(w, http.StatusNotFound, "sku_not_found", "unknown sku: "+req.SKUID)
		return
	}
	if sku.Status != "active" || !sku.IsPublic {
		httpx.Error(w, http.StatusForbidden, "sku_unavailable",
			"sku is not publicly available")
		return
	}

	uid := userIDFrom(r.Context())

	// 已经有 active 订阅 → 幂等返回。这是用户重复点"开通"的常见路径，
	// 直接 200 比 409 友好得多，UI 也不用区分两种成功路径。
	if existing, err := s.subs.GetActiveByUserSKU(r.Context(), uid, req.SKUID); err == nil && existing != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{
			"id":              existing.ID,
			"sku_id":          existing.SKUID,
			"plan_kind":       existing.PlanKind,
			"plan_name":       existing.PlanName,
			"quota_total":     existing.QuotaTotal,
			"quota_used":      existing.QuotaUsed,
			"period_end":      existing.PeriodEnd,
			"status":          existing.Status,
			"already_active":  true,
		})
		return
	} else if err != nil && !errors.Is(err, iamrepo.ErrNotFound) {
		s.logger.ErrorContext(r.Context(), "activate: lookup existing", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	now := time.Now()
	sub := &iamrepo.Subscription{
		UserID:      uid,
		SKUID:       req.SKUID,
		PlanKind:    "monthly",
		PlanName:    "自助开通",
		QuotaTotal:  math.MaxInt64, // 实际扣费走钱包；配额阀门设到无穷大
		PeriodStart: now,
		PeriodEnd:   now.AddDate(100, 0, 0), // ~永久
		AutoRenew:   true,
		QPSLimit:    10, // schema 默认
	}
	if _, err := s.subs.Create(r.Context(), sub); err != nil {
		s.logger.WarnContext(r.Context(), "activate: insert failed",
			"err", err, "user_id", uid, "sku_id", req.SKUID)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":             sub.ID,
		"sku_id":         sub.SKUID,
		"plan_kind":      sub.PlanKind,
		"plan_name":      sub.PlanName,
		"quota_total":    sub.QuotaTotal,
		"quota_used":     0,
		"period_end":     sub.PeriodEnd,
		"status":         sub.Status,
		"already_active": false,
	})
}
