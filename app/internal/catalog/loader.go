// Package catalog reconciles capability / provider / model definitions
// declared in YAML (app/configs/capabilities.yaml and
// app/configs/providers/*.yaml) into the catalog.* tables at startup.
package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

// CapabilityFile matches app/configs/capabilities.yaml.
type CapabilityFile struct {
	Capabilities []CapabilitySpec `yaml:"capabilities"`
}

// CapabilitySpec is one capability row.
type CapabilitySpec struct {
	ID          string   `yaml:"id"`
	Category    string   `yaml:"category"`
	DisplayName string   `yaml:"display_name"`
	BillingUnit string   `yaml:"billing_unit"`
	SubModes    []string `yaml:"sub_modes"`
	Transports  []string `yaml:"transports"`
}

// ProviderFile mirrors the shape of each configs/providers/*.yaml.
// The shape here drives catalog upserts; the full structure (auth,
// probe, quota_query) lives in internal/provider.Config.
type ProviderFile struct {
	ID                    string                      `yaml:"id"`
	DisplayName           string                      `yaml:"display_name"`
	BaseURL               string                      `yaml:"base_url"`
	ProtocolFamily        string                      `yaml:"protocol_family"`
	SupportedCapabilities []string                    `yaml:"supported_capabilities"`
	Auth                  struct {
		Mode string `yaml:"mode"`
	} `yaml:"auth"`
	Models map[string][]ProviderModelSpec `yaml:"models"`
}

// ProviderModelSpec is one entry inside `models.<capability>:` in a
// provider YAML file.
type ProviderModelSpec struct {
	LogicalID string                 `yaml:"logical_id"`
	Upstream  string                 `yaml:"upstream"`
	Pricing   map[string]float64     `yaml:"pricing"`
	Extras    map[string]any         `yaml:"extras,omitempty"`
}

// Loader reconciles declarative YAML into the catalog schema.
type Loader struct {
	pool *pgxpool.Pool
}

// NewLoader returns a Loader bound to a pgx pool.
func NewLoader(pool *pgxpool.Pool) *Loader { return &Loader{pool: pool} }

// Reconcile loads every YAML file under configsDir and upserts the
// catalog.* rows. Safe to run on every startup.
func (l *Loader) Reconcile(ctx context.Context, configsDir string) error {
	caps, err := readCapabilities(filepath.Join(configsDir, "capabilities.yaml"))
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	for _, c := range caps {
		if err := l.upsertCapability(ctx, c); err != nil {
			return fmt.Errorf("upsert capability %s: %w", c.ID, err)
		}
	}

	providers, err := readProviders(filepath.Join(configsDir, "providers"))
	if err != nil {
		return fmt.Errorf("providers: %w", err)
	}
	for _, p := range providers {
		if err := l.upsertProvider(ctx, p); err != nil {
			return fmt.Errorf("upsert provider %s: %w", p.ID, err)
		}
		for capID, specs := range p.Models {
			for _, m := range specs {
				if err := l.upsertLogicalModelAndMapping(ctx, capID, p.ID, m); err != nil {
					return fmt.Errorf("upsert %s/%s/%s: %w", p.ID, capID, m.LogicalID, err)
				}
			}
		}
	}
	return nil
}

func readCapabilities(path string) ([]CapabilitySpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f CapabilityFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return f.Capabilities, nil
}

func readProviders(dir string) ([]ProviderFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var p ProviderFile
		if err := yaml.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if p.ID == "" {
			return nil, fmt.Errorf("%s: missing id", e.Name())
		}
		out = append(out, p)
	}
	return out, nil
}

func (l *Loader) upsertCapability(ctx context.Context, c CapabilitySpec) error {
	const q = `
INSERT INTO catalog.capabilities (id, category, display_name, billing_unit, sub_modes, transports)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
  category = EXCLUDED.category,
  display_name = EXCLUDED.display_name,
  billing_unit = EXCLUDED.billing_unit,
  sub_modes = EXCLUDED.sub_modes,
  transports = EXCLUDED.transports
`
	_, err := l.pool.Exec(ctx, q, c.ID, c.Category, c.DisplayName, c.BillingUnit, c.SubModes, c.Transports)
	return err
}

func (l *Loader) upsertProvider(ctx context.Context, p ProviderFile) error {
	const q = `
INSERT INTO catalog.providers (id, display_name, base_url, auth_mode, protocol_family, supported_capabilities)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  base_url = EXCLUDED.base_url,
  auth_mode = EXCLUDED.auth_mode,
  protocol_family = EXCLUDED.protocol_family,
  supported_capabilities = EXCLUDED.supported_capabilities,
  updated_at = NOW()
`
	_, err := l.pool.Exec(ctx, q, p.ID, p.DisplayName, p.BaseURL, p.Auth.Mode, p.ProtocolFamily, p.SupportedCapabilities)
	return err
}

// upsertLogicalModelAndMapping creates the logical model row (if
// missing), the provider mapping, and the retail pricing snapshot — all
// in a single transaction to keep catalog views consistent.
func (l *Loader) upsertLogicalModelAndMapping(ctx context.Context, capID, providerID string, m ProviderModelSpec) error {
	if m.LogicalID == "" || m.Upstream == "" {
		return fmt.Errorf("logical_id and upstream are required")
	}
	return pgx.BeginFunc(ctx, l.pool, func(tx pgx.Tx) error {
		const upsertModel = `
INSERT INTO catalog.models (id, display_name, capability_id, category)
VALUES ($1, $1, $2, COALESCE((SELECT category FROM catalog.capabilities WHERE id = $2), 'llm'))
ON CONFLICT (id) DO UPDATE SET capability_id = EXCLUDED.capability_id
`
		if _, err := tx.Exec(ctx, upsertModel, m.LogicalID, capID); err != nil {
			return err
		}

		const upsertMapping = `
INSERT INTO catalog.model_mappings (model_id, provider_id, upstream_model, priority, status)
VALUES ($1, $2, $3, 10, 'active')
ON CONFLICT (model_id, provider_id, upstream_model) DO UPDATE SET
  priority = EXCLUDED.priority,
  status   = EXCLUDED.status
`
		if _, err := tx.Exec(ctx, upsertMapping, m.LogicalID, providerID, m.Upstream); err != nil {
			return err
		}

		in := m.Pricing["input_per_1k_cents"]
		out := m.Pricing["output_per_1k_cents"]
		unitPrice := firstNonZero(m.Pricing, "per_1k_chars_cents", "per_60_seconds_cents", "per_image_cents")
		unit := pickUnit(capID)

		const pricingIns = `
INSERT INTO catalog.pricing (model_id, provider_id, capability_id, kind, unit,
                             input_per_1k_cents, output_per_1k_cents, unit_price_cents)
SELECT $1, $2, $3, 'retail', $4, $5, $6, $7
WHERE NOT EXISTS (
  SELECT 1 FROM catalog.pricing
  WHERE model_id = $1 AND provider_id = $2 AND kind = 'retail'
    AND input_per_1k_cents = $5
    AND output_per_1k_cents = $6
    AND unit_price_cents   = $7
)
`
		_, err := tx.Exec(ctx, pricingIns, m.LogicalID, providerID, capID, unit, in, out, unitPrice)
		return err
	})
}

func firstNonZero(m map[string]float64, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok && v > 0 {
			return v
		}
	}
	return 0
}

func pickUnit(capID string) string {
	switch capID {
	case "chat", "embedding", "vlm":
		return "token"
	case "tts", "translate_text":
		return "char"
	case "asr", "translate_realtime":
		return "second"
	case "image_generation":
		return "image"
	case "ocr":
		return "page"
	}
	return "token"
}
