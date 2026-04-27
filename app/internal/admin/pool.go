package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// ----- list -----

type listAccountsResp struct {
	Data []poolrepo.AdminAccount `json:"data"`
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	f := poolrepo.AccountFilter{
		ProviderID: r.URL.Query().Get("provider"),
		Tier:       r.URL.Query().Get("tier"),
		Status:     r.URL.Query().Get("status"),
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	accs, err := s.repo.ListAdminAccounts(r.Context(), f, limit)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin list pool accounts failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, listAccountsResp{Data: accs})
}

// ----- create -----

type createAccountReq struct {
	ProviderID            string   `json:"provider_id"`
	Tier                  string   `json:"tier"`
	Origin                string   `json:"origin"`
	SupportedCapabilities []string `json:"supported_capabilities"`
	QuotaTotalCents       int64    `json:"quota_total_cents"`
	DailyLimitCents       int64    `json:"daily_limit_cents"`
	QPSLimit              int32    `json:"qps_limit"`
	CostBasisCents        int64    `json:"cost_basis_cents"`
	Tags                  []string `json:"tags"`
	IsolationGroupID      *int64   `json:"isolation_group_id,omitempty"`
	APIKeyRef             string   `json:"api_key_ref"`    // e.g. "devkey://sk-upstream-abc"
	APIKeyScope           string   `json:"api_key_scope,omitempty"`
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.ProviderID == "" || req.Tier == "" || req.Origin == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "provider_id, tier, origin are required")
		return
	}
	if req.QPSLimit == 0 {
		req.QPSLimit = 20
	}
	id, err := s.repo.CreateAdminAccount(r.Context(), poolrepo.AccountInsert{
		ProviderID:            req.ProviderID,
		Tier:                  req.Tier,
		Origin:                req.Origin,
		SupportedCapabilities: req.SupportedCapabilities,
		QuotaTotalCents:       req.QuotaTotalCents,
		DailyLimitCents:       req.DailyLimitCents,
		QPSLimit:              req.QPSLimit,
		CostBasisCents:        req.CostBasisCents,
		Tags:                  req.Tags,
		IsolationGroupID:      req.IsolationGroupID,
		APIKeyRef:             req.APIKeyRef,
		APIKeyScope:           req.APIKeyScope,
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin create pool account failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

// ----- get / patch / archive -----

func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	a, err := s.repo.GetAdminAccount(r.Context(), id)
	if errors.Is(err, poolrepo.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

type patchAccountReq struct {
	Tier            *string  `json:"tier,omitempty"`
	QuotaTotalCents *int64   `json:"quota_total_cents,omitempty"`
	DailyLimitCents *int64   `json:"daily_limit_cents,omitempty"`
	QPSLimit        *int32   `json:"qps_limit,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

func (s *Server) patchAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req patchAccountReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.repo.PatchAdminAccount(r.Context(), id, poolrepo.AdminPatch{
		Tier:            req.Tier,
		QuotaTotalCents: req.QuotaTotalCents,
		DailyLimitCents: req.DailyLimitCents,
		QPSLimit:        req.QPSLimit,
		Tags:            req.Tags,
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) archiveAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	reason := r.URL.Query().Get("reason")
	if reason == "" {
		reason = "admin_archived"
	}
	if err := s.repo.ArchiveAccount(r.Context(), id, reason); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func parseIDParam(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "invalid id")
		return 0, false
	}
	return id, true
}
