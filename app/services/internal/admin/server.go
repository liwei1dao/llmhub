// Package admin exposes the operations REST surface: pool management,
// provider catalog mutations, user support. Hosted inside the account
// service binary.
package admin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/internal/wallet"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// Server owns the admin router.
type Server struct {
	logger   *slog.Logger
	repo     *poolrepo.Repo
	iam      *iamrepo.Repo
	catalog  *catalogrepo.Repo
	metering *meteringrepo.Repo
	wallet   *wallet.Service
	token    string
}

// New builds a Server. token is a shared secret checked against the
// X-Admin-Token header; callers typically source it from env. M7 moves
// this to role-based session auth.
func New(logger *slog.Logger, repo *poolrepo.Repo, token string) *Server {
	return &Server{logger: logger, repo: repo, token: token}
}

// WithIAM plugs in the iam repo for user-admin endpoints.
func (s *Server) WithIAM(r *iamrepo.Repo) *Server { s.iam = r; return s }

// WithCatalog plugs in the catalog repo for pricing/provider views.
func (s *Server) WithCatalog(r *catalogrepo.Repo) *Server { s.catalog = r; return s }

// WithMetering plugs in the metering repo for reconciliation views.
func (s *Server) WithMetering(r *meteringrepo.Repo) *Server { s.metering = r; return s }

// WithWallet plugs in the wallet service for recharge confirmation.
func (s *Server) WithWallet(w *wallet.Service) *Server { s.wallet = w; return s }

// Mount registers /api/admin/* routes on r.
func (s *Server) Mount(r chi.Router) {
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(s.requireToken)
		r.Route("/pool/accounts", func(r chi.Router) {
			r.Get("/", s.listAccounts)
			r.Post("/", s.createAccount)
			r.Get("/{id}", s.getAccount)
			r.Patch("/{id}", s.patchAccount)
			r.Delete("/{id}", s.archiveAccount)
		})
		r.Get("/users", s.listUsers)
		r.Get("/pricing", s.listPricing)
		r.Get("/providers", s.listProviders)
		r.Get("/reconciliation", s.listRecon)
		r.Get("/pool/accounts/{id}/events", s.listAccountEvents)
		r.Post("/recharges/{order_no}/confirm", s.handleConfirmRecharge)
	})
}

func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.Header.Get("X-Admin-Token") != s.token {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "admin token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
