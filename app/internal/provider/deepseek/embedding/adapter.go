// Package embedding is the DeepSeek embedding adapter.
package embedding

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/openai_compat"
)

// Adapter implements provider.EmbeddingProvider for DeepSeek.
type Adapter struct {
	*openai_compat.EmbeddingAdapter
}

// New constructs an Adapter from provider config.
func New(cfg provider.Config) *Adapter {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.deepseek.com"
	}
	return &Adapter{
		EmbeddingAdapter: &openai_compat.EmbeddingAdapter{
			ProviderTag: "DEEPSEEK",
			BaseURL:     base,
			AuthHeader:  cfg.Auth.Header,
		},
	}
}
