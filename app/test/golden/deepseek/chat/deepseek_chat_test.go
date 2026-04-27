// Package chat runs golden protocol conformance cases against the
// DeepSeek chat adapter.
package chat

import (
	"testing"

	"github.com/llmhub/llmhub/internal/provider"
	dsChat "github.com/llmhub/llmhub/internal/provider/deepseek/chat"
	"github.com/llmhub/llmhub/test/golden/framework"
)

func TestGoldenDeepSeekChat(t *testing.T) {
	framework.RunChat(t, ".", func(baseURL string) provider.ChatProvider {
		return dsChat.New(provider.Config{
			ID:      "deepseek",
			BaseURL: baseURL,
			Auth: provider.AuthConfig{
				Mode:   "bearer",
				Header: "Authorization",
			},
			ErrorMapping: map[string]string{
				"RateLimitExceeded":   "rate_limited",
				"InsufficientBalance": "depleted",
				"InvalidAPIKey":       "auth_failed",
			},
		})
	})
}
