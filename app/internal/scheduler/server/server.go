// Package server is the HTTP front-end of the scheduler service.
//
// Hosts a small RPC surface (Pick / Report) used by the gateway hot
// path. The transport is HTTP/JSON today; the wire schema mirrors
// proto/scheduler/v1 so the eventual gRPC migration is mechanical.
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/scheduler"
	"github.com/llmhub/llmhub/internal/scheduler/rpc"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// Server hosts scheduler RPCs.
type Server struct {
	logger *slog.Logger
	svc    *scheduler.Service
	token  string
}

// New constructs a Server. token is the shared secret callers must
// present in X-Internal-Token; empty token means "reject all".
func New(logger *slog.Logger, svc *scheduler.Service, token string) *Server {
	return &Server{logger: logger, svc: svc, token: token}
}

// Router builds the chi router with middleware + RPC routes.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/rpc/scheduler", func(r chi.Router) {
		r.Use(s.requireToken)
		r.Post("/Pick", s.handlePick)
		r.Post("/Report", s.handleReport)
	})
	return r
}

func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.Header.Get("X-Internal-Token") != s.token {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "internal token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handlePick(w http.ResponseWriter, r *http.Request) {
	var req rpc.PickRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	res, err := s.svc.Pick(r.Context(), scheduler.PickRequest{
		RequestID:         req.RequestID,
		UserID:            req.UserID,
		CapabilityID:      req.CapabilityID,
		ProviderID:        req.ProviderID,
		ModelID:           req.ModelID,
		EstimatedUnits:    req.EstimatedUnits,
		RiskLevel:         domain.RiskLevel(req.RiskLevel),
		SessionKey:        req.SessionKey,
		ExcludeAccountIDs: req.ExcludeAccountIDs,
	})
	if err != nil {
		// UnifiedError carries kind + code; pass through if available.
		if ue, ok := err.(*domain.UnifiedError); ok && ue.Kind == domain.ErrNoAccountAvailable {
			httpx.Error(w, http.StatusServiceUnavailable, "no_account_available", ue.Message)
			return
		}
		s.logger.ErrorContext(r.Context(), "scheduler pick failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rpc.PickResponse{
		AccountID:  res.AccountID,
		ProviderID: res.ProviderID,
		Tier:       string(res.Tier),
		PickToken:  res.PickToken,
	})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var req rpc.ReportRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.AccountID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "account_id is required")
		return
	}
	if err := s.svc.Report(r.Context(), req.AccountID, scheduler.ReportResult(req.Result)); err != nil {
		s.logger.ErrorContext(r.Context(), "scheduler report failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rpc.Empty{})
}

// ErrNoAccountAvailable is exported for parity with the local
// scheduler.Service signature when callers want to type-assert.
var ErrNoAccountAvailable = errors.New("scheduler: no account available")
