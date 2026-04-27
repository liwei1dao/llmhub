// Package chat runs golden protocol conformance cases against the
// Anthropic chat adapter.
package chat

import (
	"testing"

	"github.com/llmhub/llmhub/internal/provider"
	anthChat "github.com/llmhub/llmhub/internal/provider/anthropic/chat"
	"github.com/llmhub/llmhub/test/golden/framework"
)

func TestGoldenAnthropicChat(t *testing.T) {
	framework.RunChat(t, ".", func(baseURL string) provider.ChatProvider {
		return anthChat.New(provider.Config{
			ID:      "anthropic",
			BaseURL: baseURL,
			Auth: provider.AuthConfig{
				Mode:   "bearer",
				Header: "x-api-key",
			},
		})
	})
}
