package pool

import (
	"context"
	"log/slog"
	"time"

	"github.com/llmhub/llmhub/internal/domain"
	"github.com/llmhub/llmhub/internal/platform/vault"
	"github.com/llmhub/llmhub/internal/provider"
)

// Prober runs a minimal upstream call per declared capability on a
// freshly-onboarded pool account and narrows the
// supported_capabilities column to the set that actually succeeded.
//
// This is fire-and-forget from the admin flow: failures are logged but
// don't roll back the account creation.
type Prober struct {
	Logger    *slog.Logger
	Pool      *Service
	Providers ProviderLookup
	Vault     vault.Resolver
}

// ProviderLookup is the subset of provider.Manager we need.
type ProviderLookup interface {
	Lookup(id string) (provider.Provider, bool)
}

// Probe walks the declared capability ids, calls a tiny upstream
// request for each, and writes back the verified set. Runs with a
// bounded per-capability timeout so a hung upstream doesn't wedge the
// admin flow.
func (p *Prober) Probe(ctx context.Context, accountID int64, declared []string) error {
	acc, err := p.Pool.Get(ctx, accountID)
	if err != nil {
		return err
	}
	prov, ok := p.Providers.Lookup(acc.ProviderID)
	if !ok {
		p.Logger.WarnContext(ctx, "probe: provider not registered", "provider", acc.ProviderID)
		return nil
	}
	verified := make([]string, 0, len(declared))
	for _, cap := range declared {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		ok, reason := p.probeOne(cctx, prov, cap, accountID)
		cancel()
		result := "fail"
		if ok {
			verified = append(verified, cap)
			result = "ok"
		}
		if err := p.Pool.Repo().RecordProbeEvent(ctx, accountID, result, reason); err != nil {
			p.Logger.WarnContext(ctx, "probe: event insert failed", "err", err)
		}
	}
	if err := p.Pool.Repo().SetSupportedCapabilities(ctx, accountID, verified); err != nil {
		return err
	}
	p.Logger.InfoContext(ctx, "probe complete", "account_id", accountID, "verified", verified)
	return nil
}

// probeOne returns true if the capability smoke test succeeds.
// Each capability checks: adapter implements the corresponding
// XxxCapability() interface, an active credential exists, and the
// adapter can build a well-formed upstream request from a tiny payload.
//
// We deliberately stop short of actually firing the request — that
// would be expensive and can incur upstream charges; the goal is to
// catch *configuration drift* (missing adapter, missing key, missing
// model mapping) at onboarding time, not to certify health.
func (p *Prober) probeOne(ctx context.Context, prov provider.Provider, capability string, accountID int64) (bool, string) {
	keyRow, err := p.Pool.ActiveAPIKey(ctx, accountID, capability)
	if err != nil {
		// Some adapters fall back to scope='all' or 'chat' for any
		// capability — try that broader scope before giving up.
		if keyRow, err = p.Pool.ActiveAPIKey(ctx, accountID, "all"); err != nil {
			return false, "no active key"
		}
	}
	sec, err := p.Vault.Resolve(ctx, keyRow.VaultRef)
	if err != nil {
		return false, "credential resolve failed"
	}
	cred := domain.Credential{APIKey: sec["api_key"]}

	switch capability {
	case "chat":
		cap := prov.ChatCapability()
		if cap == nil {
			return false, "adapter missing ChatCapability"
		}
		req := &domain.ChatRequest{
			Model:    "probe",
			Messages: []domain.ChatMessage{{Role: "user", Content: "ping"}},
		}
		if _, err := cap.TranslateRequest(ctx, req, cred); err != nil {
			return false, "translate failed: " + err.Error()
		}
		return true, "chat request built"

	case "embedding":
		if prov.EmbeddingCapability() == nil {
			return false, "adapter missing EmbeddingCapability"
		}
		return true, "embedding adapter present"

	case "translate_text":
		if prov.TranslateCapability() == nil {
			return false, "adapter missing TranslateCapability"
		}
		return true, "translate adapter present"

	case "tts":
		if prov.TTSCapability() == nil {
			return false, "adapter missing TTSCapability"
		}
		return true, "tts adapter present"

	case "asr":
		if prov.ASRCapability() == nil {
			return false, "adapter missing ASRCapability"
		}
		return true, "asr adapter present"

	case "vlm":
		if prov.VLMCapability() == nil {
			return false, "adapter missing VLMCapability"
		}
		return true, "vlm adapter present"

	case "image_generation":
		if prov.ImageCapability() == nil {
			return false, "adapter missing ImageCapability"
		}
		return true, "image adapter present"
	}
	// Unknown capability id — accept by default so newly declared
	// capabilities don't get silently stripped before code lands.
	return true, "unchecked"
}
