// Package deepseek is the adapter for DeepSeek's OpenAI-compatible API.
//
// DeepSeek's chat-completions and embeddings endpoints are near drop-ins
// for OpenAI, so the concrete capability adapters are thin compositions
// over openai_compat.* under deepseek/chat and deepseek/embedding.
package deepseek

import (
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/deepseek/chat"
	"github.com/llmhub/llmhub/internal/provider/deepseek/embedding"
)

// ID is the provider identifier used by the catalog and registry.
const ID = "deepseek"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return newAdapter(cfg), nil
	})
}

type adapter struct {
	cfg       provider.Config
	chat      *chat.Adapter
	embedding *embedding.Adapter
}

func newAdapter(cfg provider.Config) *adapter {
	return &adapter{
		cfg:       cfg,
		chat:      chat.New(cfg),
		embedding: embedding.New(cfg),
	}
}

func (a *adapter) ID() string { return ID }

func (a *adapter) Meta() domain.ProviderMeta {
	return domain.ProviderMeta{
		ID:                    a.cfg.ID,
		DisplayName:           a.cfg.DisplayName,
		BaseURL:               a.cfg.BaseURL,
		AuthMode:              a.cfg.Auth.Mode,
		ProtocolFamily:        a.cfg.ProtocolFamily,
		Status:                "active",
		SupportedCapabilities: a.cfg.SupportedCapabilities,
	}
}

func (a *adapter) ChatCapability() provider.ChatProvider           { return a.chat }
func (a *adapter) EmbeddingCapability() provider.EmbeddingProvider { return a.embedding }
func (a *adapter) ASRCapability() provider.ASRProvider             { return nil }
func (a *adapter) TTSCapability() provider.TTSProvider             { return nil }
func (a *adapter) TranslateCapability() provider.TranslateProvider { return nil }
func (a *adapter) ImageCapability() provider.ImageProvider         { return nil }
func (a *adapter) VLMCapability() provider.VLMProvider             { return nil }
