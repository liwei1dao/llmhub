// Package errcode centralises the mapping from domain.ErrorKind to HTTP
// status codes and stable LLMH_* codes. Concrete handlers convert
// *domain.UnifiedError into responses using this table.
package errcode

import (
	"net/http"

	"github.com/llmhub/llmhub/internal/domain"
)

// HTTPStatus returns the default HTTP status for an error kind.
func HTTPStatus(k domain.ErrorKind) int {
	switch k {
	case domain.ErrInvalidRequest:
		return http.StatusBadRequest
	case domain.ErrUnauthorized:
		return http.StatusUnauthorized
	case domain.ErrScopeForbidden:
		return http.StatusForbidden
	case domain.ErrInsufficientBalance:
		return http.StatusPaymentRequired
	case domain.ErrModelNotFound:
		return http.StatusNotFound
	case domain.ErrModelDeprecated:
		return http.StatusConflict
	case domain.ErrRateLimited:
		return http.StatusTooManyRequests
	case domain.ErrUpstreamError:
		return http.StatusBadGateway
	case domain.ErrUpstreamTimeout:
		return http.StatusGatewayTimeout
	case domain.ErrNoAccountAvailable:
		return http.StatusServiceUnavailable
	case domain.ErrInternal:
		return http.StatusInternalServerError
	}
	return http.StatusInternalServerError
}
