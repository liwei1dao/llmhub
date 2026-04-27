// Package server is the HTTP front-end of the billing service.
//
// It exposes a narrow RPC-style surface — Freeze / Settle / Release —
// used by the gateway before and after each AI call. Protected by a
// shared-secret header until mTLS is introduced in M8.
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/llmhub/llmhub/internal/billing/rpc"
	"github.com/llmhub/llmhub/internal/wallet"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// Server hosts billing RPCs.
type Server struct {
	logger *slog.Logger
	wallet *wallet.Service
	token  string
}

// New constructs a Server. token is the shared secret callers must
// present in X-Internal-Token; empty token means "reject all" so
// production can't accidentally boot with an open billing endpoint.
func New(logger *slog.Logger, w *wallet.Service, token string) *Server {
	return &Server{logger: logger, wallet: w, token: token}
}

// Router builds the chi router with middleware + RPC routes.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/rpc/billing", func(r chi.Router) {
		r.Use(s.requireToken)
		r.Post("/Freeze", s.handleFreeze)
		r.Post("/Settle", s.handleSettle)
		r.Post("/Release", s.handleRelease)
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

func (s *Server) handleFreeze(w http.ResponseWriter, r *http.Request) {
	var req rpc.FreezeRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.wallet.Freeze(r.Context(), req.RequestID, req.UserID, req.Cents); err != nil {
		// Insufficient funds is a business outcome, not a transport error.
		if errors.Is(err, wallet.ErrInsufficientFunds) {
			httpx.JSON(w, http.StatusOK, rpc.FreezeResponse{Accepted: false, Reason: "insufficient_balance"})
			return
		}
		s.logger.ErrorContext(r.Context(), "billing freeze failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rpc.FreezeResponse{Accepted: true})
}

func (s *Server) handleSettle(w http.ResponseWriter, r *http.Request) {
	var req rpc.SettleRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.wallet.Settle(r.Context(), req.RequestID, req.ActualCents); err != nil {
		s.logger.ErrorContext(r.Context(), "billing settle failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rpc.Empty{})
}

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	var req rpc.ReleaseRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.wallet.Release(r.Context(), req.RequestID); err != nil {
		s.logger.ErrorContext(r.Context(), "billing release failed", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rpc.Empty{})
}
