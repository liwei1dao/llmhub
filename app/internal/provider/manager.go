package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Manager owns the runtime set of concrete Provider instances.
//
// Factories are registered at program start via init() in each
// internal/provider/<id>/adapter.go. Manager then walks a configs
// directory, for each YAML finds the matching factory, builds the
// Provider, and caches it for O(1) lookup on the gateway hot path.
//
// Rebuild on catalog changes happens through Reload; callers can grab a
// stable snapshot with Snapshot().
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewManager returns an empty Manager.
func NewManager() *Manager { return &Manager{providers: make(map[string]Provider)} }

// Lookup returns the Provider registered under id, if any.
func (m *Manager) Lookup(id string) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[id]
	return p, ok
}

// Snapshot returns a shallow copy of the current id→provider map.
func (m *Manager) Snapshot() map[string]Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]Provider, len(m.providers))
	for k, v := range m.providers {
		out[k] = v
	}
	return out
}

// IDs returns the ids of every loaded provider.
func (m *Manager) IDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.providers))
	for k := range m.providers {
		ids = append(ids, k)
	}
	return ids
}

// LoadDir walks a directory of YAML provider definitions and builds a
// Provider instance for each id that has a registered factory. Files
// without a matching factory are logged to stderr and skipped — this
// lets operators introduce a new YAML before the Go adapter lands.
func (m *Manager) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read providers dir: %w", err)
	}
	built := make(map[string]Provider)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		cfg, err := loadConfig(path)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		factory, ok := Providers.Get(cfg.ID)
		if !ok {
			// Provider YAML exists but no Go adapter registered; not fatal.
			fmt.Fprintf(os.Stderr, "provider/manager: no factory registered for %s; skipping\n", cfg.ID)
			continue
		}
		prov, err := factory(cfg)
		if err != nil {
			return fmt.Errorf("instantiate provider %s: %w", cfg.ID, err)
		}
		built[cfg.ID] = prov
	}
	m.mu.Lock()
	m.providers = built
	m.mu.Unlock()
	return nil
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.ID == "" {
		return cfg, fmt.Errorf("provider yaml missing id")
	}
	return cfg, nil
}
