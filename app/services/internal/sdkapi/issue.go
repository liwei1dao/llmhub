package sdkapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
)

// IssueRequest is the SDK → platform request body for /sdk/credentials/issue.
type IssueRequest struct {
	SKUID             string `json:"sku_id"`
	ClientFingerprint string `json:"client_fingerprint,omitempty"` // optional, for risk control
}

// IssueResponse is what the SDK gets back. AuthPayload is the *real*
// upstream credential (api_key / app_token / ak+sk / ...). The SDK is
// responsible for never exposing it to user code and clearing it on
// process exit.
type IssueResponse struct {
	LeaseID        string            `json:"lease_id"`
	ExpiresAt      time.Time         `json:"expires_at"`
	IssuedAt       time.Time         `json:"issued_at"`
	Vendor         string            `json:"vendor"`         // catalog.Vendors id
	VendorProduct  string            `json:"vendor_product"` // catalog.Products id
	Capability     string            `json:"capability"`
	UpstreamModel  string            `json:"upstream_model,omitempty"`
	Endpoint       string            `json:"endpoint"`        // base URL the SDK should hit
	ProtocolFamily string            `json:"protocol_family"` // hint to the SDK's adapter switch
	AuthPayload    map[string]string `json:"auth_payload"`    // sensitive — never logged, never persisted on the SDK side
}

// handleIssue mints a lease and returns real upstream credentials.
//
// Validation chain (each failure is its own status code so the SDK can
// surface a user-friendly message):
//
//   1. Bearer auth → user_id + api_key_id (401 if absent / bad)
//   2. SKU exists + active                (404 / 410)
//   3. User has active subscription        (403 not_subscribed)
//   4. Subscription has remaining quota    (402 quota_exceeded)
//   5. PickBinding finds a healthy binding (503 no_binding_available)
//   6. Vault resolves auth_payload         (500 vault_error)
//   7. INSERT pool.leases                  (500 internal)
//
// Step 5 uses the same pool.Service.PickBinding the gateway used to use;
// the difference is the result is handed to the SDK instead of being
// invoked in-process.
func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	auth, err := s.authenticate(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if req.SKUID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "sku_id is required")
		return
	}

	// Step 2: SKU.
	sku, err := s.d.Catalog.LookupSKU(r.Context(), req.SKUID)
	if err != nil {
		if errors.Is(err, catalogrepo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "sku_not_found", "unknown sku: "+req.SKUID)
			return
		}
		s.d.Logger.ErrorContext(r.Context(), "issue: sku lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "sku lookup failed")
		return
	}
	if sku.Status != "active" {
		writeError(w, http.StatusGone, "sku_deprecated", "sku is not active: "+sku.Status)
		return
	}

	// Step 3 + 4: subscription + quota.
	sub, err := s.d.Subs.GetActiveByUserSKU(r.Context(), auth.UserID, req.SKUID)
	if err != nil {
		if errors.Is(err, iamrepo.ErrNotFound) {
			writeError(w, http.StatusForbidden, "not_subscribed",
				"user has no active subscription for sku: "+req.SKUID)
			return
		}
		s.d.Logger.ErrorContext(r.Context(), "issue: subscription lookup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "subscription lookup failed")
		return
	}
	if sub.QuotaUsed >= sub.QuotaTotal {
		writeError(w, http.StatusPaymentRequired, "quota_exceeded",
			"subscription quota exhausted; please upgrade or wait for renewal")
		return
	}
	if sub.DailyQuotaLimit != nil && sub.DailyUsed >= *sub.DailyQuotaLimit {
		writeError(w, http.StatusPaymentRequired, "daily_quota_exceeded",
			"daily quota exhausted")
		return
	}

	// Step 5: pick a binding under (vendor_product, capability).
	picks, err := s.d.Pool.PickBinding(r.Context(), sku.VendorProductID, sku.Capability, 40)
	if err != nil {
		s.d.Logger.ErrorContext(r.Context(), "issue: pick binding", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "binding pick failed")
		return
	}
	if len(picks) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no_binding_available",
			"no healthy upstream binding available; the operator has been notified")
		return
	}
	chosen := picks[0]

	// Step 6: vault.
	authPayload, err := s.d.Vault.Resolve(r.Context(), chosen.AuthPayloadRef)
	if err != nil {
		s.d.Logger.ErrorContext(r.Context(), "issue: vault resolve", "err", err, "ref", chosen.AuthPayloadRef)
		writeError(w, http.StatusInternalServerError, "vault_error", "credential resolution failed")
		return
	}
	if len(authPayload) == 0 {
		writeError(w, http.StatusInternalServerError, "vault_error", "credential resolved to empty payload")
		return
	}

	// Step 7: persist lease.
	now := time.Now()
	expires := now.Add(time.Duration(s.d.LeaseTTLSec) * time.Second)
	clientIP := parseClientIP(r)
	lease := &poolrepo.Lease{
		UserID:            auth.UserID,
		APIKeyID:          auth.APIKeyID,
		SKUID:             sku.ID,
		BindingID:         chosen.BindingID,
		CredentialID:      chosen.CredentialID,
		ClientFingerprint: req.ClientFingerprint,
		ClientIP:          clientIP,
		ExpiresAt:         expires,
	}
	leaseUUID, err := s.d.Pool.Repo().Leases().Create(r.Context(), lease)
	if err != nil {
		s.d.Logger.ErrorContext(r.Context(), "issue: lease insert", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "lease persist failed")
		return
	}

	// Resolve endpoint + protocol_family from the static catalog. Falling
	// back to whatever's in the SKU's product entry keeps the SDK self-
	// contained — it only needs lease + endpoint + auth, not full DB access.
	endpoint, proto := lookupProductHints(chosen.ProductID)

	resp := IssueResponse{
		LeaseID:        leaseUUID,
		ExpiresAt:      expires,
		IssuedAt:       lease.IssuedAt,
		Vendor:         chosen.VendorID,
		VendorProduct:  chosen.ProductID,
		Capability:     chosen.Capability,
		UpstreamModel:  sku.UpstreamModel,
		Endpoint:       endpoint,
		ProtocolFamily: proto,
		AuthPayload:    authPayload,
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, resp)
}

// parseClientIP best-effort extracts the SDK caller's IP. RealIP middleware
// (if mounted) already normalizes X-Forwarded-For; otherwise we strip the
// port off RemoteAddr.
func parseClientIP(r *http.Request) *net.IP {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if ip := net.ParseIP(host); ip != nil {
		return &ip
	}
	return nil
}

// lookupProductHints fetches the static catalog entry for a product and
// returns (endpoint_template, protocol_family). The SDK uses both: the
// endpoint to know where to POST, the protocol_family to choose its
// internal adapter (openai_compat / volc_signed_v4 / etc.).
func lookupProductHints(productID string) (endpoint, protocol string) {
	// Imported lazily via a tiny seam to avoid a hard dep on catalog package
	// at the top-level (the wiring tests fake this).
	if p, ok := lookupProductFn(productID); ok {
		return p.Endpoint, p.Protocol
	}
	return "", ""
}

// productHints decouples the issue handler from the catalog package's
// concrete VendorProduct shape so tests can swap in a fake.
type productHints struct {
	Endpoint string
	Protocol string
}

var lookupProductFn = func(_ string) (productHints, bool) { return productHints{}, false }

// ProductHinter is set by main.go (or wiring code) so the issue handler
// can resolve endpoint/protocol from catalog without circular imports.
type ProductHinter func(productID string) (endpoint string, protocol string, ok bool)

// SetProductHinter installs the catalog accessor.
func SetProductHinter(h ProductHinter) {
	lookupProductFn = func(id string) (productHints, bool) {
		ep, pf, ok := h(id)
		return productHints{Endpoint: ep, Protocol: pf}, ok
	}
}

// authResult is the (user_id, api_key_id) pair returned by authenticate.
type authResult struct {
	UserID   int64
	APIKeyID int64
}

// authenticate verifies the Bearer token. The SDK is expected to send
// "Authorization: Bearer <key>" where <key> is the plaintext api_key
// issued at signup. We don't currently require a separate "id" field
// because api_key already uniquely identifies the user; the (id, key)
// formulation is the user-facing terminology.
func (s *Server) authenticate(r *http.Request) (*authResult, error) {
	bearer := parseBearer(r)
	if bearer == "" {
		return nil, errMissingAuth
	}
	userID, keyID, err := s.d.Auth.AuthenticateAPIKey(r.Context(), bearer)
	if err != nil || userID == 0 {
		return nil, errBadAuth
	}
	return &authResult{UserID: userID, APIKeyID: keyID}, nil
}

func parseBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

var (
	errMissingAuth = errors.New("missing bearer token")
	errBadAuth     = errors.New("invalid api key")
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"type": code, "message": message},
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch err {
	case errMissingAuth:
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
	default:
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid api key")
	}
}

// silence unused import on cold paths
var _ = context.Background
