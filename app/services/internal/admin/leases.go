package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// LeaseView is the admin-facing JSON shape for a pool.leases row.
// auth_payload_ref is deliberately not exposed — admin shouldn't see
// (and doesn't need to see) which vault path the lease points at.
type LeaseView struct {
	LeaseID           string  `json:"lease_id"`
	UserID            int64   `json:"user_id"`
	APIKeyID          int64   `json:"api_key_id"`
	SKUID             string  `json:"sku_id"`
	BindingID         int64   `json:"binding_id"`
	CredentialID      int64   `json:"credential_id"`
	ClientFingerprint string  `json:"client_fingerprint,omitempty"`
	ClientIP          string  `json:"client_ip,omitempty"`
	Status            string  `json:"status"`
	IssuedAt          string  `json:"issued_at"`
	ExpiresAt         string  `json:"expires_at"`
	RevokedAt         *string `json:"revoked_at,omitempty"`
	RevokeReason      string  `json:"revoke_reason,omitempty"`
	LastUsedAt        *string `json:"last_used_at,omitempty"`
	UseCount          int32   `json:"use_count"`
	TotalInputUnits   int64   `json:"total_input_units"`
	TotalOutputUnits  int64   `json:"total_output_units"`
}

func toLeaseView(l *poolrepo.Lease) LeaseView {
	v := LeaseView{
		LeaseID:           l.LeaseID,
		UserID:            l.UserID,
		APIKeyID:          l.APIKeyID,
		SKUID:             l.SKUID,
		BindingID:         l.BindingID,
		CredentialID:      l.CredentialID,
		ClientFingerprint: l.ClientFingerprint,
		Status:            l.Status,
		IssuedAt:          l.IssuedAt.Format(time.RFC3339),
		ExpiresAt:         l.ExpiresAt.Format(time.RFC3339),
		RevokeReason:      l.RevokeReason,
		UseCount:          l.UseCount,
		TotalInputUnits:   l.TotalInputUnits,
		TotalOutputUnits:  l.TotalOutputUnits,
	}
	if l.ClientIP != nil {
		v.ClientIP = l.ClientIP.String()
	}
	if l.RevokedAt != nil {
		s := l.RevokedAt.Format(time.RFC3339)
		v.RevokedAt = &s
	}
	if l.LastUsedAt != nil {
		s := l.LastUsedAt.Format(time.RFC3339)
		v.LastUsedAt = &s
	}
	return v
}

// listLeases handles GET /api/admin/leases.
//
// Query string: ?user_id=...&sku_id=...&binding_id=...&status=...&active=1
//
// active=1 is shorthand for "status=active AND expires_at>NOW()" — the
// most common operations view ("show me everything currently usable
// across the whole platform").
func (s *Server) listLeases(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	q := r.URL.Query()
	uid, _ := strconv.ParseInt(q.Get("user_id"), 10, 64)
	bid, _ := strconv.ParseInt(q.Get("binding_id"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	rows, err := s.pool.Repo().Leases().List(r.Context(), poolrepo.LeaseFilter{
		UserID:     uid,
		SKUID:      q.Get("sku_id"),
		BindingID:  bid,
		Status:     q.Get("status"),
		OnlyActive: q.Get("active") == "1",
		Limit:      limit,
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin list leases", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]LeaseView, 0, len(rows))
	for i := range rows {
		out = append(out, toLeaseView(&rows[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// revokeLease handles DELETE /api/admin/leases/{lease_id}.
//
// The SDK's next /sdk/usage/report or any binding-pick path that needs
// this lease will get back 401 / not_found; the platform's lease
// pick path doesn't reuse revoked leases.
func (s *Server) revokeLease(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	leaseID := chi.URLParam(r, "lease_id")
	if leaseID == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "lease_id is required")
		return
	}
	reason := r.URL.Query().Get("reason")
	if reason == "" {
		reason = "admin_revoke"
	}
	if err := s.pool.Repo().Leases().Revoke(r.Context(), leaseID, reason); err != nil {
		if errors.Is(err, poolrepo.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "lease not found or already revoked")
			return
		}
		s.logger.ErrorContext(r.Context(), "admin revoke lease",
			"err", err, "lease_id", leaseID)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.recordAdmin(r, "revoke_lease", "lease", leaseID, "ok", map[string]any{"reason": reason})
	w.WriteHeader(http.StatusNoContent)
}
