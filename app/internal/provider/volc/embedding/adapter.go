// Package embedding is the Volcengine Ark embedding adapter.
package embedding

import (
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/openai_compat"
)

// Adapter implements provider.EmbeddingProvider for Volc Ark.
type Adapter struct {
	*openai_compat.EmbeddingAdapter
}

// New constructs an Adapter from provider config.
func New(cfg provider.Config) *Adapter {
	base := cfg.BaseURL
	if base == "" {
		base = "https://ark.cn-beijing.volces.com/api/v3"
	}
	return &Adapter{
		EmbeddingAdapter: &openai_compat.EmbeddingAdapter{
			ProviderTag: "VOLC",
			BaseURL:     base,
			AuthHeader:  cfg.Auth.Header,
		},
	}
}
