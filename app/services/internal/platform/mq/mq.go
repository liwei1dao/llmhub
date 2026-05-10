// Package mq provides a NATS JetStream client used for asynchronous
// inter-service events (call.completed, pool.state_changed, ...).
package mq

import (
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/llmhub/llmhub/internal/platform/config"
)

// Open connects to NATS.
func Open(cfg config.NATSConfig) (*nats.Conn, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("nats.url is required")
	}
	nc, err := nats.Connect(cfg.URL,
		nats.Name("llmhub"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return nc, nil
}

// Subject builds a standard llmhub event subject.
// Example: Subject("call", "completed", "v1") -> "llmhub.call.completed.v1".
func Subject(domain, action, version string) string {
	return fmt.Sprintf("llmhub.%s.%s.%s", domain, action, version)
}
