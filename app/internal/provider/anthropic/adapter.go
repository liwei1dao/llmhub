// Package anthropic is the adapter for Anthropic's Messages API.
package anthropic

import (
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/anthropic/chat"
)

// ID is the provider identifier used by the catalog and registry.
const ID = "anthropic"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return newAdapter(cfg)
	})
}

type adapter struct {
	cfg  provider.Config
	chat *chat.Adapter
}

func newAdapter(cfg provider.Config) (*adapter, error) {
	return &adapter{cfg: cfg, chat: chat.New(cfg)}, nil
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
func (a *adapter) EmbeddingCapability() provider.EmbeddingProvider { return nil }
func (a *adapter) ASRCapability() provider.ASRProvider             { return nil }
func (a *adapter) TTSCapability() provider.TTSProvider             { return nil }
func (a *adapter) TranslateCapability() provider.TranslateProvider { return nil }
func (a *adapter) ImageCapability() provider.ImageProvider         { return nil }
func (a *adapter) VLMCapability() provider.VLMProvider             { return nil }
