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

// CredentialView is the JSON shape for a credential row. Auth refs and
// payload digest are deliberately omitted from the wire — admins look
// at the credential through the wizard / vault, not via the list API.
type CredentialView struct {
	ID                  int64   `json:"id"`
	VendorID            string  `json:"vendor_id"`
	AccountID           int64   `json:"account_id"`
	ProductID           string  `json:"product_id"`
	Name                string  `json:"name"`
	Env                 string  `json:"env"`
	Status              string  `json:"status"`
	HealthScore         int16   `json:"health_score"`
	IsolationGroupID    *int64  `json:"isolation_group_id,omitempty"`
	ConsecutiveFailures int32   `json:"consecutive_failures"`
	CooldownUntil       *string `json:"cooldown_until,omitempty"`
	LastUsedAt          *string `json:"last_used_at,omitempty"`
	LastErrorAt         *string `json:"last_error_at,omitempty"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func toCredentialView(c *poolrepo.Credential) CredentialView {
	v := CredentialView{
		ID:                  c.ID,
		VendorID:            c.VendorID,
		AccountID:           c.AccountID,
		ProductID:           c.ProductID,
		Name:                c.Name,
		Env:                 c.Env,
		Status:              c.Status,
		HealthScore:         c.HealthScore,
		IsolationGroupID:    c.IsolationGroupID,
		ConsecutiveFailures: c.ConsecutiveFailures,
		CreatedAt:           c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if c.CooldownUntil != nil {
		s := c.CooldownUntil.Format("2006-01-02T15:04:05Z07:00")
		v.CooldownUntil = &s
	}
	if c.LastUsedAt != nil {
		s := c.LastUsedAt.Format("2006-01-02T15:04:05Z07:00")
		v.LastUsedAt = &s
	}
	if c.LastErrorAt != nil {
		s := c.LastErrorAt.Format("2006-01-02T15:04:05Z07:00")
		v.LastErrorAt = &s
	}
	return v
}

// BindingView is the JSON shape for a service binding (调度行).
type BindingView struct {
	ID              int64   `json:"id"`
	CredentialID    int64   `json:"credential_id"`
	Capability      string  `json:"capability"`
	Tier            string  `json:"tier"`
	QPSLimit        *int32  `json:"qps_limit,omitempty"`
	DailyLimitCents *int64  `json:"daily_limit_cents,omitempty"`
	QuotaTotalCents *int64  `json:"quota_total_cents,omitempty"`
	DailyUsedCents  int64   `json:"daily_used_cents"`
	CostBasisCents  float64 `json:"cost_basis_cents"`
	HealthScore     int16   `json:"health_score"`
	Status          string  `json:"status"`
}

func toBindingView(b *poolrepo.ServiceBinding) BindingView {
	return BindingView{
		ID:              b.ID,
		CredentialID:    b.CredentialID,
		Capability:      b.Capability,
		Tier:            b.Tier,
		QPSLimit:        b.QPSLimit,
		DailyLimitCents: b.DailyLimitCents,
		QuotaTotalCents: b.QuotaTotalCents,
		DailyUsedCents:  b.DailyUsedCents,
		CostBasisCents:  b.CostBasisCents,
		HealthScore:     b.HealthScore,
		Status:          b.Status,
	}
}

// listCredentials handles GET /api/admin/credentials.
func (s *Server) listCredentials(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("account_id"), 10, 64)
	rows, err := s.pool.Repo().Credentials().List(r.Context(), poolrepo.CredentialFilter{
		VendorID:  r.URL.Query().Get("vendor"),
		ProductID: r.URL.Query().Get("product"),
		AccountID: accountID,
		Status:    r.URL.Query().Get("status"),
		Search:    r.URL.Query().Get("q"),
		Limit:     limit,
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "admin list credentials", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]CredentialView, 0, len(rows))
	for i := range rows {
		out = append(out, toCredentialView(&rows[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// createCredentialReq is the body for the wizard's final submit. Auth
// payload follows the product's CredentialSchema; bindings are the
// services the user picked + their per-binding quota / cost figures.
type createCredentialReq struct {
	AccountID        int64                  `json:"account_id"`
	ProductID        string                 `json:"product_id"`
	Name             string                 `json:"name"`
	Env              string                 `json:"env"`
	IsolationGroupID *int64                 `json:"isolation_group_id,omitempty"`
	AuthPayload      map[string]string      `json:"auth_payload"`
	Bindings         []credentialBindingReq `json:"bindings"`
}
type credentialBindingReq struct {
	Capability      string  `json:"capability"`
	Tier            string  `json:"tier"`
	QPSLimit        *int32  `json:"qps_limit,omitempty"`
	DailyLimitCents *int64  `json:"daily_limit_cents,omitempty"`
	QuotaTotalCents *int64  `json:"quota_total_cents,omitempty"`
	CostBasisCents  float64 `json:"cost_basis_cents"`
}

type createCredentialResp struct {
	Credential CredentialView `json:"credential"`
	Bindings   []BindingView  `json:"bindings"`
}

// createCredential handles POST /api/admin/credentials.
//
// Behaviour: validates static-catalog invariants, writes the auth
// payload to vault (placeholder), then delegates to the pool service
// which performs INSERT credentials + N INSERT credential_services
// in a single transaction.
func (s *Server) createCredential(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	var req createCredentialReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.AccountID == 0 || req.ProductID == "" || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"account_id, product_id, name are required")
		return
	}
	if len(req.Bindings) == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request",
			"at least one binding is required")
		return
	}

	// TODO(M-vault): write req.AuthPayload to vault, capture path + digest.
	authRef := "devcred://pending"
	if len(req.AuthPayload) > 0 {
		authRef = "devcred://" + req.ProductID + "/" + req.Name
	}

	bindings := make([]pool.CredentialBindingInput, 0, len(req.Bindings))
	for _, b := range req.Bindings {
		bindings = append(bindings, pool.CredentialBindingInput{
			Capability:      b.Capability,
			Tier:            b.Tier,
			QPSLimit:        b.QPSLimit,
			DailyLimitCents: b.DailyLimitCents,
			QuotaTotalCents: b.QuotaTotalCents,
			CostBasisCents:  b.CostBasisCents,
		})
	}

	cred, bs, err := s.pool.CreateCredential(r.Context(), pool.CreateCredentialInput{
		AccountID:        req.AccountID,
		ProductID:        req.ProductID,
		Name:             req.Name,
		Env:              req.Env,
		AuthPayloadRef:   authRef,
		IsolationGroupID: req.IsolationGroupID,
		Bindings:         bindings,
	})
	switch {
	case errors.Is(err, pool.ErrUnknownProduct),
		errors.Is(err, pool.ErrUnknownVendor),
		errors.Is(err, pool.ErrVendorMismatch),
		errors.Is(err, pool.ErrCapabilityNotAllowed):
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	case errors.Is(err, poolrepo.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		return
	case err != nil:
		s.logger.ErrorContext(r.Context(), "admin create credential", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := createCredentialResp{Credential: toCredentialView(cred)}
	for _, b := range bs {
		resp.Bindings = append(resp.Bindings, toBindingView(b))
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// getCredential handles GET /api/admin/credentials/{id}. Returns the
// credential row + every binding it owns.
func (s *Server) getCredential(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	c, err := s.pool.Repo().Credentials().Get(r.Context(), id)
	if errors.Is(err, poolrepo.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "credential not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	bs, err := s.pool.Repo().Bindings().ListByCredential(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	bvs := make([]BindingView, 0, len(bs))
	for i := range bs {
		bvs = append(bvs, toBindingView(&bs[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"credential": toCredentialView(c),
		"bindings":   bvs,
	})
}

// archiveCredential handles DELETE /api/admin/credentials/{id}.
func (s *Server) archiveCredential(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	if err := s.pool.Repo().Credentials().UpdateStatus(r.Context(), id, "archived"); err != nil {
		if errors.Is(err, poolrepo.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "credential not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addBindingReq is the request body for "+ 加服务" — attach one new
// binding to an existing credential.
type addBindingReq struct {
	Capability      string  `json:"capability"`
	Tier            string  `json:"tier"`
	QPSLimit        *int32  `json:"qps_limit,omitempty"`
	DailyLimitCents *int64  `json:"daily_limit_cents,omitempty"`
	QuotaTotalCents *int64  `json:"quota_total_cents,omitempty"`
	CostBasisCents  float64 `json:"cost_basis_cents"`
}

// addBinding handles POST /api/admin/credentials/{id}/bindings.
func (s *Server) addBinding(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	var req addBindingReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Capability == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "capability required")
		return
	}
	b, err := s.pool.AddBinding(r.Context(), pool.AddBindingInput{
		CredentialID:    id,
		Capability:      req.Capability,
		Tier:            req.Tier,
		QPSLimit:        req.QPSLimit,
		DailyLimitCents: req.DailyLimitCents,
		QuotaTotalCents: req.QuotaTotalCents,
		CostBasisCents:  req.CostBasisCents,
	})
	switch {
	case errors.Is(err, pool.ErrCapabilityNotAllowed):
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	case errors.Is(err, poolrepo.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "credential not found")
		return
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, toBindingView(b))
}

// listCredentialEvents handles GET /api/admin/credentials/{id}/events.
func (s *Server) listCredentialEvents(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "pool service not wired")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "id must be int")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	evs, err := s.pool.Repo().Events().ListByCredential(r.Context(), id, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": evs})
}
