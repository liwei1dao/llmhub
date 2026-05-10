package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/internal/pool"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// VendorAccountView is the JSON shape for a master account row. Not
// the same as repo.VendorAccount — auth refs are intentionally NOT
// returned to keep credential paths off the wire.
type VendorAccountView struct {
	ID                  int64   `json:"id"`
	VendorID            string  `json:"vendor_id"`
	Name                string  `json:"name"`
	Entity              string  `json:"entity,omitempty"`
	ConsoleURL          string  `json:"console_url,omitempty"`
	Status              string  `json:"status"`
	LastBalanceCents    *int64  `json:"last_balance_cents,omitempty"`
	LastBalanceCurrency string  `json:"last_balance_currency,omitempty"`
	LastBalanceAt       *string `json:"last_balance_at,omitempty"`
	LastBalanceError    string  `json:"last_balance_error,omitempty"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func toVendorAccountView(a *poolrepo.VendorAccount) VendorAccountView {
	v := VendorAccountView{
		ID:                  a.ID,
		VendorID:            a.VendorID,
		Name:                a.Name,
		Entity:              a.Entity,
		ConsoleURL:          a.ConsoleURL,
		Status:              a.Status,
		LastBalanceCents:    a.LastBalanceCents,
		LastBalanceCurrency: a.LastBalanceCurrency,
		LastBalanceError:    a.LastBalanceError,
		CreatedAt:           a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if a.LastBalanceAt != nil {
		s := a.LastBalanceAt.Format("2006-01-02T15:04:05Z07:00")
		v.LastBalanceAt = &s
	}
	return v
}

// listVendorAccounts handles GET /api/admin/vendor-accounts.
func (s *Server) listVendorAccounts(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.pool.Repo().VendorAccounts().List(r.Context(), poolrepo.VendorAccountFilter{
		VendorID: r.URL.Query().Get("vendor"),
		Status:   r.URL.Query().Get("status"),
		Search:   r.URL.Query().Get("q"),
		Limit:    limit,
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin list vendor accounts", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]VendorAccountView, 0, len(rows))
	for i := range rows {
		out = append(out, toVendorAccountView(&rows[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// createVendorAccountReq is the request body for POST.
//
// auth_payload is the raw key-value bag matching the vendor's
// MasterAuthSchema. The handler turns it into a vault ref before
// persistence — DB never sees the secret material.
type createVendorAccountReq struct {
	VendorID    string            `json:"vendor_id"`
	Name        string            `json:"name"`
	Entity      string            `json:"entity"`
	ConsoleURL  string            `json:"console_url"`
	AuthPayload map[string]string `json:"auth_payload"`
}

// createVendorAccount handles POST /api/admin/vendor-accounts.
//
// In production a real vault writer would be plumbed in; for now the
// handler just synthesises a ref-string so end-to-end flows can be
// exercised.
func (s *Server) createVendorAccount(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	var req createVendorAccountReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.VendorID == "" || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "vendor_id and name required")
		return
	}
	// TODO(M-vault): write req.AuthPayload to vault and capture the path.
	// For now use a deterministic placeholder so smoke flows pass.
	authRef := "vault://pool/vendor_accounts/pending"
	if len(req.AuthPayload) > 0 {
		authRef = "devmaster://" + req.VendorID + "/" + req.Name
	}
	a, err := s.pool.CreateVendorAccount(r.Context(), pool.CreateVendorAccountInput{
		VendorID:      req.VendorID,
		Name:          req.Name,
		Entity:        req.Entity,
		ConsoleURL:    req.ConsoleURL,
		MasterAuthRef: authRef,
	})
	if err != nil {
		if errors.Is(err, pool.ErrUnknownVendor) {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		s.logger.ErrorContext(r.Context(), "admin create vendor account", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toVendorAccountView(a))
}

// getVendorAccount handles GET /api/admin/vendor-accounts/{id}.
func (s *Server) getVendorAccount(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	a, err := s.pool.Repo().VendorAccounts().Get(r.Context(), id)
	if errors.Is(err, poolrepo.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "vendor_account not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, toVendorAccountView(a))
}

// patchVendorAccountReq mutates the writable subset of a vendor account.
// Only `status` is mutable in v0.2; renames and re-pointing console_url
// are rare enough to defer to a follow-up.
type patchVendorAccountReq struct {
	Status string `json:"status,omitempty"`
}

// patchVendorAccount handles PATCH /api/admin/vendor-accounts/{id}.
func (s *Server) patchVendorAccount(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	var req patchVendorAccountReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Status != "" {
		switch req.Status {
		case "active", "frozen", "archived":
		default:
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "bad status")
			return
		}
		if err := s.pool.Repo().VendorAccounts().UpdateStatus(r.Context(), id, req.Status); err != nil {
			if errors.Is(err, poolrepo.ErrNotFound) {
				httpx.Error(w, http.StatusNotFound, "not_found", "vendor_account not found")
				return
			}
			httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// archiveVendorAccount handles DELETE /api/admin/vendor-accounts/{id}.
// "Archive" is a status flip — the row is preserved for historical
// reconciliation queries.
func (s *Server) archiveVendorAccount(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	if err := s.pool.Repo().VendorAccounts().UpdateStatus(r.Context(), id, "archived"); err != nil {
		if errors.Is(err, poolrepo.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "vendor_account not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
