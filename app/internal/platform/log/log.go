// Package log provides a structured logger shared across services.
//
// Wraps log/slog with a JSON handler by default. Request-scoped values
// (request_id, user_id, trace_id) are carried via context and can be
// attached with FromContext.
package log

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyUserID
	ctxKeyTraceID
	ctxKeyAPIKeyID
)

// New returns a service-scoped logger.
func New(service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(h).With("service", service)
}

// WithRequestID returns a context with the given request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// WithUserID returns a context with the given user id.
func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// WithTraceID returns a context with the given trace id.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyTraceID, id)
}

// WithAPIKeyID returns a context with the given API key id.
func WithAPIKeyID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyAPIKeyID, id)
}

// FromContext returns a logger with request-scoped fields attached.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok && v != "" {
		l = l.With("request_id", v)
	}
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok && v != "" {
		l = l.With("trace_id", v)
	}
	if v, ok := ctx.Value(ctxKeyUserID).(int64); ok && v != 0 {
		l = l.With("user_id", v)
	}
	if v, ok := ctx.Value(ctxKeyAPIKeyID).(int64); ok && v != 0 {
		l = l.With("api_key_id", v)
	}
	return l
}
