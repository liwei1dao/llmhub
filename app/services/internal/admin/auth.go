package admin

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/llmhub/llmhub/internal/adminauth"
	"github.com/llmhub/llmhub/pkg/httpx"
)

// adminCtxKey is the context key for the resolved admin id.
type adminCtxKey struct{}

func adminIDFrom(ctx context.Context) int64 {
	v, _ := ctx.Value(adminCtxKey{}).(int64)
	return v
}

// requireAdmin replaces the previous shared-token middleware. It looks
// up X-Admin-Token in adminauth.sessions and injects the admin id into
// the request context for downstream handlers.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			httpx.Error(w, http.StatusInternalServerError, "internal_error", "adminauth not wired")
			return
		}
		token := r.Header.Get("X-Admin-Token")
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		adminID, err := s.auth.ResolveSession(r.Context(), token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "登录态已失效，请重新登录")
			return
		}
		ctx := context.WithValue(r.Context(), adminCtxKey{}, adminID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loginReq is the body of POST /api/admin/auth/login.
type loginReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// handleAdminLogin: account + password → session token.
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "adminauth not wired")
		return
	}
	var req loginReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Account == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "account / password 必填")
		return
	}

	a, err := s.auth.Login(r.Context(), req.Account, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, adminauth.ErrInvalidCredentials):
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "账号或密码错误")
		case errors.Is(err, adminauth.ErrAdminDisabled):
			httpx.Error(w, http.StatusForbidden, "forbidden", "账号已停用")
		default:
			s.logger.ErrorContext(r.Context(), "admin login error", "err", err)
			httpx.Error(w, http.StatusInternalServerError, "internal_error", "登录失败")
		}
		return
	}

	clientIP := r.RemoteAddr
	if h, _, splitErr := net.SplitHostPort(clientIP); splitErr == nil {
		clientIP = h
	}
	sess, err := s.auth.IssueSession(r.Context(), a.ID, r.UserAgent(), clientIP)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "issue admin session", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "签发会话失败")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"token":      sess.Token,
		"expires_at": sess.ExpiresAt,
		"admin": map[string]any{
			"id":           a.ID,
			"account":      a.Account,
			"display_name": a.DisplayName,
		},
	})
}

// handleAdminLogout revokes the current session.
func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "adminauth not wired")
		return
	}
	if err := s.auth.RevokeSessionByToken(r.Context(), r.Header.Get("X-Admin-Token")); err != nil {
		s.logger.ErrorContext(r.Context(), "admin logout", "err", err)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAdminMe returns the currently logged-in admin profile, used by
// the front-end to verify that a stored token is still valid.
func (s *Server) handleAdminMe(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		httpx.Error(w, http.StatusNotImplemented, "internal_error", "adminauth not wired")
		return
	}
	id := adminIDFrom(r.Context())
	if id <= 0 {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "未登录")
		return
	}
	a, err := s.auth.Repo().GetAdminByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":            a.ID,
		"account":       a.Account,
		"display_name":  a.DisplayName,
		"status":        a.Status,
		"last_login_at": a.LastLoginAt,
		"created_at":    a.CreatedAt,
	})
}
