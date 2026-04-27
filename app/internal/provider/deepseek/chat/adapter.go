// Package chat is the DeepSeek chat adapter. DeepSeek's /v1/chat/completions
// endpoint is a near drop-in for OpenAI, so the concrete adapter is
// just a thin composition over openai_compat.ChatAdapter.
package chat

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/openai_compat"
)

// Adapter implements provider.ChatProvider for DeepSeek.
type Adapter struct {
	*openai_compat.ChatAdapter
}

// New constructs an Adapter from provider config.
func New(cfg provider.Config) *Adapter {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.deepseek.com"
	}
	header := cfg.Auth.Header
	if header == "" {
		header = "Authorization"
	}
	return &Adapter{
		ChatAdapter: &openai_compat.ChatAdapter{
			ProviderTag: "DEEPSEEK",
			BaseURL:     base,
			AuthHeader:  header,
			ErrorMap:    cfg.ErrorMapping,
		},
	}
}
