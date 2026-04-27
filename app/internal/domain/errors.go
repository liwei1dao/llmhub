package domain

import "fmt"

// ErrorKind is the platform-wide error classification. Every error
// returned from service/handler code should map to one of these.
type ErrorKind string

const (
	ErrInvalidRequest      ErrorKind = "invalid_request"
	ErrUnauthorized        ErrorKind = "unauthorized"
	ErrScopeForbidden      ErrorKind = "scope_forbidden"
	ErrInsufficientBalance ErrorKind = "insufficient_balance"
	ErrModelNotFound       ErrorKind = "model_not_found"
	ErrModelDeprecated     ErrorKind = "model_deprecated"
	ErrRateLimited         ErrorKind = "rate_limited"
	ErrUpstreamError       ErrorKind = "upstream_error"
	ErrUpstreamTimeout     ErrorKind = "upstream_timeout"
	ErrNoAccountAvailable  ErrorKind = "no_account_available"
	ErrInternal            ErrorKind = "internal_error"
)

// UnifiedError is the internal error type carrying enough context for
// both structured logging and user-facing responses.
type UnifiedError struct {
	Kind      ErrorKind
	Code      string // e.g. "LLMH_429_001"
	Message   string
	Retryable bool
	Cause     error
	Meta      map[string]any
}

func (e *UnifiedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *UnifiedError) Unwrap() error { return e.Cause }

// New constructs a UnifiedError.
func NewError(kind ErrorKind, code, message string) *UnifiedError {
	return &UnifiedError{Kind: kind, Code: code, Message: message}
}

// WithCause attaches an underlying error.
func (e *UnifiedError) WithCause(err error) *UnifiedError {
	e.Cause = err
	return e
}

// WithMeta attaches metadata to the error.
func (e *UnifiedError) WithMeta(k string, v any) *UnifiedError {
	if e.Meta == nil {
		e.Meta = make(map[string]any, 1)
	}
	e.Meta[k] = v
	return e
}
