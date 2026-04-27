// Package chat runs golden protocol conformance cases against the
// Volcengine Ark chat adapter.
package chat

import (
	"testing"

	"github.com/llmhub/llmhub/internal/provider"
	volcchat "github.com/llmhub/llmhub/internal/provider/volc/chat"
	"github.com/llmhub/llmhub/test/golden/framework"
)

func TestGoldenVolcChat(t *testing.T) {
	framework.RunChat(t, ".", func(baseURL string) provider.ChatProvider {
		return volcchat.New(provider.Config{
			ID:      "volc",
			BaseURL: baseURL,
			Auth: provider.AuthConfig{
				Mode:   "bearer",
				Header: "Authorization",
			},
			ErrorMapping: map[string]string{
				"RateLimitExceeded":   "rate_limited",
				"InsufficientBalance": "depleted",
				"AccountFrozen":       "banned",
			},
		})
	})
}
