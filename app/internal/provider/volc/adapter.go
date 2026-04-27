// Package volc is the adapter for Volcengine Ark.
//
// This is the reference adapter showcasing the sub-package layout:
//
//	volc/
//	  adapter.go       <- this file: registration + capability aggregation
//	  config.go
//	  auth/            <- shared auth/signer
//	  errors/          <- shared error-code mapping
//	  chat/            <- chat capability adapter
//	  asr/             <- asr capability adapter
//	  tts/             <- tts capability adapter
//	  ...
package volc

import (
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
	"github.com/llmhub/llmhub/internal/provider/volc/chat"
	"github.com/llmhub/llmhub/internal/provider/volc/embedding"
)

// ID is the provider identifier used by the catalog and registry.
const ID = "volc"

func init() {
	provider.Providers.Register(ID, func(cfg provider.Config) (provider.Provider, error) {
		return newAdapter(cfg)
	})
}

type adapter struct {
	cfg       provider.Config
	chat      *chat.Adapter
	embedding *embedding.Adapter
}

func newAdapter(cfg provider.Config) (*adapter, error) {
	return &adapter{
		cfg:       cfg,
		chat:      chat.New(cfg),
		embedding: embedding.New(cfg),
	}, nil
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

func (a *adapter) ChatCapability() provider.ChatProvider       { return a.chat }
func (a *adapter) EmbeddingCapability() provider.EmbeddingProvider { return a.embedding }
func (a *adapter) ASRCapability() provider.ASRProvider         { return nil } // TODO(M6)
func (a *adapter) TTSCapability() provider.TTSProvider         { return nil } // TODO(M6)
func (a *adapter) TranslateCapability() provider.TranslateProvider { return nil }
func (a *adapter) ImageCapability() provider.ImageProvider     { return nil }
func (a *adapter) VLMCapability() provider.VLMProvider         { return nil }
