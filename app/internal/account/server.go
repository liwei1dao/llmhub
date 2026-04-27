// Package account hosts the HTTP server that powers both the end-user
// console (`/api/user/*`) and the ops admin (`/api/admin/*`) behind the
// same binary.
package account

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/llmhub/llmhub/internal/admin"
	"github.com/llmhub/llmhub/internal/iam"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/wallet"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// SessionCookieName is the cookie key carrying the session token.
const SessionCookieName = "llmhub_session"

// Server wires the chi router against the account-bounded services.
// Keep this small — handlers live in their own files as the surface grows.
type Server struct {
	logger   *slog.Logger
	iam      *iam.Service
	wallet   *wallet.Service
	metering *meteringrepo.Repo
	sessions *iam.MemSessionIndex
	admin    *admin.Server
	mux      http.Handler
	pinger   Pinger
}

// Pinger is implemented by *pgxpool.Pool. It is the minimal contract
// /ready uses to confirm the database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// New constructs a configured account server.
func New(logger *slog.Logger, iamSvc *iam.Service, walletSvc *wallet.Service, adminSrv *admin.Server) *Server {
	s := &Server{
		logger:   logger,
		iam:      iamSvc,
		wallet:   walletSvc,
		sessions: iam.NewMemSessionIndex(),
		admin:    adminSrv,
	}
	s.mux = s.routes()
	return s
}

// WithMetering attaches a metering repo so /api/user/usage/* endpoints
// can read aggregated call data. Returns the same Server for chaining.
func (s *Server) WithMetering(r *meteringrepo.Repo) *Server {
	s.metering = r
	s.mux = s.routes()
	return s
}

// WithPinger attaches a database health probe so /ready can report a
// real liveness signal. Returns the same Server for chaining.
func (s *Server) WithPinger(p Pinger) *Server {
	s.pinger = p
	return s
}

// Handler returns the composed http.Handler (router + middleware).
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(devCORS)

	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	if s.admin != nil {
		s.admin.Mount(r)
	}

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)
			r.Get("/profile", s.handleProfile)
			r.Get("/wallet", s.handleWallet)
			r.Get("/usage/series", s.handleUsageSeries)
			r.Get("/api-keys", s.handleListAPIKeys)
			r.Post("/api-keys", s.handleCreateAPIKey)
			r.Post("/wallet/recharge", s.handleCreateRecharge)
			r.Get("/wallet/recharges", s.handleListRecharges)
			r.Get("/wallet/recharges/{order_no}", s.handleGetRecharge)
		})
	})
	return r
}

// -------- health --------

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.pinger != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.pinger.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "db_unreachable", err.Error())
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// -------- auth --------

type registerReq struct {
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name,omitempty"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "password is required")
		return
	}
	u, err := s.iam.Register(r.Context(), iam.RegisterRequest{
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		s.logger.WarnContext(r.Context(), "register failed", "err", err)
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := s.wallet.EnsureAccount(r.Context(), u.ID); err != nil {
		s.logger.ErrorContext(r.Context(), "wallet bootstrap failed", "err", err, "user_id", u.ID)
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":     u.ID,
		"email":  u.Email,
		"status": u.Status,
	})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	u, err := s.iam.LoginByEmail(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, iam.ErrInvalidCredentials) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	sess, err := s.iam.IssueSession(r.Context(), u.ID, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "issue session failed", "err", err, "user_id", u.ID)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "cannot issue session")
		return
	}
	s.sessions.Put(sess.Token, u.ID, sess.ExpiresAt)
	setSessionCookie(w, sess.Token, sess.ExpiresAt)

	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":     u.ID,
		"email":  u.Email,
		"status": u.Status,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil {
		s.sessions.Delete(c.Value)
	}
	clearSessionCookie(w)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   false, // dev; prod behind TLS should override via reverse proxy
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// -------- authenticated --------

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	u, err := s.iam.GetUser(r.Context(), uid)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":        u.ID,
		"email":     u.Email,
		"phone":     u.Phone,
		"status":    u.Status,
		"qps_limit": u.QPSLimit,
	})
}

func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	acc, err := s.wallet.EnsureAccount(r.Context(), uid)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"balance_cents":         acc.BalanceCents,
		"frozen_cents":          acc.FrozenCents,
		"currency":              acc.Currency,
		"total_recharged_cents": acc.TotalRechargedCents,
		"total_spent_cents":     acc.TotalSpentCents,
	})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	keys, err := s.iam.ListAPIKeys(r.Context(), uid)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{
			"id":      k.ID,
			"prefix":  k.Prefix,
			"name":    k.Name,
			"scopes":  k.Scopes,
			"status":  k.Status,
			"created": k.CreatedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

type createKeyReq struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	uid := userIDFrom(r.Context())
	var req createKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	res, err := s.iam.CreateAPIKey(r.Context(), iam.CreateAPIKeyRequest{
		UserID: uid,
		Name:   req.Name,
		Scopes: req.Scopes,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":     res.Key.ID,
		"prefix": res.Key.Prefix,
		"key":    res.Plain,
	})
}

// -------- middleware --------

type ctxKey int

const ctxKeyUser ctxKey = iota

// requireUser is the auth middleware for /api/user/* (post-register).
// It prefers the session cookie issued at login; as a dev fallback it
// accepts an X-User-Id header (removed before prod).
func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := s.resolveUser(r)
		if uid == 0 {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "login required")
			return
		}
		next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), uid)))
	})
}

func (s *Server) resolveUser(r *http.Request) int64 {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		if uid := s.sessions.Get(c.Value); uid != 0 {
			return uid
		}
		if uid, err := s.iam.ResolveSession(r.Context(), c.Value); err == nil && uid != 0 {
			return uid
		}
	}
	return parseUserHeader(r)
}

func parseUserHeader(r *http.Request) int64 {
	v := r.Header.Get("X-User-Id")
	if v == "" {
		return 0
	}
	var n int64
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

func withUserID(ctx context.Context, uid int64) context.Context {
	return context.WithValue(ctx, ctxKeyUser, uid)
}

// devCORS reflects the request origin so the local Next.js apps
// (console :3001, admin :3002) can hit the account service on :8081
// during development. The list of allowed origins is fed from the
// LLMHUB_WEB_ORIGINS env var (comma-separated). Empty means open to all
// localhost origins, which is appropriate for dev-only environments.
//
// Production deployments terminate CORS at the reverse proxy.
func devCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Token, X-Internal-Token, X-User-Id")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowOrigin(origin string) bool {
	allowed := strings.Split(os.Getenv("LLMHUB_WEB_ORIGINS"), ",")
	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return true
		}
	}
	// Open to localhost / 127.0.0.1 by default for dev convenience.
	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:")
}

func userIDFrom(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyUser).(int64); ok {
		return v
	}
	return 0
}
