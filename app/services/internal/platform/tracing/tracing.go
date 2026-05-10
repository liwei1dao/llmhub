// Package tracing is a thin wrapper around OpenTelemetry for distributed
// tracing. Services call Init at startup and Shutdown on exit.
//
// Keep this stub minimal until M8; full OTLP wiring lands there.
package tracing

import (
	"context"

	"github.com/llmhub/llmhub/internal/platform/config"
)

// Init is a placeholder for OTLP tracer setup.
// Returns a shutdown function that must be deferred by the caller.
func Init(_ context.Context, _ config.TracingConfig, _ string) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}
