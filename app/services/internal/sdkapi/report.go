package sdkapi

import (
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
	if req.LeaseID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "lease_id is required")
		return
	}
	if req.Status == "" {
		req.Status = "success"
	}

	// Step 2 + 3: load the lease and cross-check ownership.
	lease, err := s.d.Pool.Repo().Leases().GetActive(r.Context(), req.LeaseID)
	if err != nil {
		if errors.Is(err, poolrepo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "lease_not_found",
				"lease is unknown, expired, or revoked")
			return
		}
		s.d.Logger.ErrorContext(r.Context(), "report: lease lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "lease lookup failed")
		return
	}
	if lease.UserID != auth.UserID {
		writeError(w, http.StatusForbidden, "lease_owner_mismatch",
			"lease is owned by a different user")
		return
	}

	// Step 4: append metering.call_logs (best-effort).
	s.appendCallLog(r, &req, lease)

	// Step 5: bump lease counters.
	if err := s.d.Pool.Repo().Leases().AddUsage(r.Context(), req.LeaseID, req.InputUnits, req.OutputUnits); err != nil {
		s.d.Logger.WarnContext(r.Context(), "report: lease usage update", "err", err)
	}

	// Step 6: decrement subscription quota. Total units = input+output.
	// For usage-report ergonomics, we count whichever side the SKU bills
	// on; for llm SKUs both sides count.
	total := req.InputUnits + req.OutputUnits
	if total > 0 {
		if _, _, err := s.d.Subs.AddUsage(r.Context(), lease.UserID, lease.SKUID, total); err != nil {
			if !errors.Is(err, iamrepo.ErrNotFound) {
				s.d.Logger.WarnContext(r.Context(), "report: quota decrement", "err", err)
			}
		}
	}

	// Step 7: health-score feedback. Only failures move the needle; on
	// success the binding's score slowly recovers via successful reports
	// (caps at 100 in the SQL).
	outcome := mapReportOutcome(req.Status)
	if delta := deltaForOutcome(outcome); delta != 0 {
		if _, err := s.d.Pool.Repo().Bindings().AdjustHealth(r.Context(), lease.BindingID, delta, req.ErrorCode); err != nil {
			s.d.Logger.WarnContext(r.Context(), "report: binding health", "err", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
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

// appendCallLog writes a metering row from the report. Best-effort —
// usage reports are a soft signal, so we log failures and swallow them.
//
// vendor_id / product_id come from the upstream binding context that
// the lease was originally minted under. Admin reports join back to
// pool.credentials when they need the full vendor lineage.
func (s *Server) appendCallLog(r *http.Request, req *UsageReportRequest, lease *poolrepo.Lease) {
	if s.d.Metering == nil {
		return
	}
	// Resolve vendor_id + product_id from the SKU. The SKU is cached in
	// catalog.Service so this is essentially free.
	var vendorID, productID string
	if sku, err := s.d.Catalog.LookupSKU(r.Context(), lease.SKUID); err == nil && sku != nil {
		productID = sku.VendorProductID
		// vendor_id is the prefix of "<vendor>.<product>" — keeps SDK API
		// from having to import the static catalog Products map.
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
	if err := s.d.Metering.InsertCallLogV2(r.Context(), row); err != nil {
		s.d.Logger.WarnContext(r.Context(), "report: metering append", "err", err)
	}
}
