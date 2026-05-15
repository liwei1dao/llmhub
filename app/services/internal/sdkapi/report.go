package sdkapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
)

// reportOutcome enumerates the SDK-reported call results that drive
// binding-health adjustments. Inlined here (instead of a shared scheduler
// package) because in the 聚合 SDK 平台 model there is no scheduler — the
// SDK is the only thing scheduling upstream calls.
type reportOutcome string

const (
	outcomeSuccess     reportOutcome = "success"
	outcomeRateLimited reportOutcome = "rate_limited"
	outcomeAuthFailed  reportOutcome = "auth_failed"
	outcomeTimeout     reportOutcome = "timeout"
	outcomeUpstreamErr reportOutcome = "upstream_error"
)

// UsageReportRequest is what the SDK posts after each completed upstream
// call. The platform never saw the call itself, so this report is the
// only signal we have for billing, health-tracking, and observability.
//
// The SDK MUST send this best-effort: we tolerate occasional drops for
// network reasons, but consistent missing reports for a (user, sku)
// trigger anomaly alerts in admin.
type UsageReportRequest struct {
	LeaseID    string `json:"lease_id"`
	RequestID  string `json:"request_id,omitempty"` // SDK-generated, for de-dup
	// Usage in SKU.billing_unit terms. Only the relevant fields populate.
	InputUnits  int64 `json:"input_units,omitempty"`  // 1k_tokens / minute / page / image
	OutputUnits int64 `json:"output_units,omitempty"` // llm output side
	// Outcome
	Status     string `json:"status"`                // success / upstream_error / timeout / rate_limited / auth_failed
	ErrorCode  string `json:"error_code,omitempty"`  // e.g. "LLMH_429_VOLC_ARK"
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	TTFBMs     int64  `json:"ttfb_ms,omitempty"`     // streaming only
}

// handleUsageReport ingests one usage event from the SDK. The handler:
//
//   1. Verifies the SDK's bearer token (same auth as /issue)
//   2. Resolves lease_id → active lease row
//   3. Cross-checks lease.user_id matches the bearer's user_id
//   4. INSERT metering.call_logs
//   5. Increments lease.use_count + token totals
//   6. Decrements iam.subscriptions.quota_used
//   7. Adjusts pool.credential_services.health_score on failure
//
// Best-effort: we always 204 on partial failures (logging the error)
// because the SDK has nothing useful to do with a 5xx here.
func (s *Server) handleUsageReport(w http.ResponseWriter, r *http.Request) {
	auth, err := s.authenticate(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req UsageReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if status, code, msg, ok := s.IngestUsage(r.Context(), auth.UserID, &req); !ok {
		writeError(w, status, code, msg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// IngestUsage is the core of the SDK's POST /sdk/usage/report — it
// looks up the lease, verifies ownership, and writes the metering /
// quota / health-score side-effects. Exported so the user-console test
// path (cookie-auth) can record its own usage without rewriting the
// pipeline. Returns (status, errCode, message, ok); ok=true means
// nothing went wrong and the caller can 204 / continue.
func (s *Server) IngestUsage(ctx context.Context, ownerUserID int64, req *UsageReportRequest) (int, string, string, bool) {
	if req.LeaseID == "" {
		return http.StatusBadRequest, "invalid_request", "lease_id is required", false
	}
	if req.Status == "" {
		req.Status = "success"
	}

	lease, err := s.d.Pool.Repo().Leases().GetActive(ctx, req.LeaseID)
	if err != nil {
		if errors.Is(err, poolrepo.ErrNotFound) {
			return http.StatusNotFound, "lease_not_found", "lease is unknown, expired, or revoked", false
		}
		s.d.Logger.ErrorContext(ctx, "report: lease lookup", "err", err)
		return http.StatusInternalServerError, "internal_error", "lease lookup failed", false
	}
	if lease.UserID != ownerUserID {
		return http.StatusForbidden, "lease_owner_mismatch", "lease is owned by a different user", false
	}

	s.appendCallLogCtx(ctx, req, lease)

	if err := s.d.Pool.Repo().Leases().AddUsage(ctx, req.LeaseID, req.InputUnits, req.OutputUnits); err != nil {
		s.d.Logger.WarnContext(ctx, "report: lease usage update", "err", err)
	}

	total := req.InputUnits + req.OutputUnits
	if total > 0 {
		if _, _, err := s.d.Subs.AddUsage(ctx, lease.UserID, lease.SKUID, total); err != nil {
			if !errors.Is(err, iamrepo.ErrNotFound) {
				s.d.Logger.WarnContext(ctx, "report: quota decrement", "err", err)
			}
		}
	}

	outcome := mapReportOutcome(req.Status)
	if delta := deltaForOutcome(outcome); delta != 0 {
		if _, err := s.d.Pool.Repo().Bindings().AdjustHealth(ctx, lease.BindingID, delta, req.ErrorCode); err != nil {
			s.d.Logger.WarnContext(ctx, "report: binding health", "err", err)
		}
	}
	return 0, "", "", true
}

// mapReportOutcome normalises an SDK-reported status string. Unknown /
// empty values count as success — the SDK is the only source of truth
// and we don't want a typo on the client to silently demote a binding.
func mapReportOutcome(status string) reportOutcome {
	switch status {
	case "", "success":
		return outcomeSuccess
	case "rate_limited":
		return outcomeRateLimited
	case "auth_failed":
		return outcomeAuthFailed
	case "timeout":
		return outcomeTimeout
	}
	return outcomeUpstreamErr
}

// deltaForOutcome returns the health-score delta. Success nudges +1
// (slow recovery), failures drop by amounts matched to severity:
// rate_limited is transient (-10), auth_failed is structural (-50),
// timeout / generic upstream error is between (-5).
func deltaForOutcome(o reportOutcome) int {
	switch o {
	case outcomeSuccess:
		return +1
	case outcomeRateLimited:
		return -10
	case outcomeAuthFailed:
		return -50
	case outcomeTimeout, outcomeUpstreamErr:
		return -5
	}
	return 0
}

// appendCallLogCtx writes a metering row from the report. Best-effort —
// usage reports are a soft signal, so we log failures and swallow them.
//
// vendor_id / product_id come from the upstream binding context that
// the lease was originally minted under. Admin reports join back to
// pool.credentials when they need the full vendor lineage.
func (s *Server) appendCallLogCtx(ctx context.Context, req *UsageReportRequest, lease *poolrepo.Lease) {
	if s.d.Metering == nil {
		return
	}
	var vendorID, productID string
	if sku, err := s.d.Catalog.LookupSKU(ctx, lease.SKUID); err == nil && sku != nil {
		productID = sku.VendorProductID
		if idx := strings.IndexByte(productID, '.'); idx > 0 {
			vendorID = productID[:idx]
		}
	}
	row := meteringrepo.CallLogV2{
		Timestamp:    time.Now().UTC(),
		RequestID:    req.RequestID,
		LeaseID:      req.LeaseID,
		UserID:       lease.UserID,
		APIKeyID:     lease.APIKeyID,
		SKUID:        lease.SKUID,
		VendorID:     vendorID,
		ProductID:    productID,
		BindingID:    lease.BindingID,
		CredentialID: lease.CredentialID,
		InputUnits:   req.InputUnits,
		OutputUnits:  req.OutputUnits,
		Status:       req.Status,
		ErrorCode:    req.ErrorCode,
		DurationMs:   req.LatencyMs,
		TTFBMs:       req.TTFBMs,
	}
	if err := s.d.Metering.InsertCallLogV2(ctx, row); err != nil {
		s.d.Logger.WarnContext(ctx, "report: metering append", "err", err)
	}
}
