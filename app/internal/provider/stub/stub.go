// Package stub provides a no-op Provider implementation used as a
// placeholder while real adapters are being built out. Each provider
// package under internal/provider/<id>/ constructs one of these so it
// can register with the global provider registry even before its
// concrete capability adapters exist.
package stub

import (
	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/provider"
)

// Provider is a no-op provider that reports zero capabilities.
type Provider struct {
	cfg provider.Config
}

// New returns a new stub provider seeded with cfg.
func New(cfg provider.Config) *Provider { return &Provider{cfg: cfg} }

func (p *Provider) ID() string { return p.cfg.ID }

func (p *Provider) Meta() domain.ProviderMeta {
	return domain.ProviderMeta{
		ID:                    p.cfg.ID,
		DisplayName:           p.cfg.DisplayName,
		BaseURL:               p.cfg.BaseURL,
		AuthMode:              p.cfg.Auth.Mode,
		ProtocolFamily:        p.cfg.ProtocolFamily,
		Status:                "paused",
		SupportedCapabilities: p.cfg.SupportedCapabilities,
	}
}

func (p *Provider) ChatCapability() provider.ChatProvider           { return nil }
func (p *Provider) EmbeddingCapability() provider.EmbeddingProvider { return nil }
func (p *Provider) ASRCapability() provider.ASRProvider             { return nil }
func (p *Provider) TTSCapability() provider.TTSProvider             { return nil }
func (p *Provider) TranslateCapability() provider.TranslateProvider { return nil }
func (p *Provider) ImageCapability() provider.ImageProvider         { return nil }
func (p *Provider) VLMCapability() provider.VLMProvider             { return nil }
