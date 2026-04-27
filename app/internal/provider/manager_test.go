package provider_test

import (
	"path/filepath"
	"testing"

	"github.com/llmhub/llmhub/internal/provider"

	// Register known factories; volc exposes a real ChatProvider, the
	// rest are stubs.
	_ "github.com/llmhub/llmhub/internal/provider/anthropic"
	_ "github.com/llmhub/llmhub/internal/provider/dashscope"
	_ "github.com/llmhub/llmhub/internal/provider/deepseek"
	_ "github.com/llmhub/llmhub/internal/provider/volc"
)

// TestManagerLoadsBundledConfigs walks the real configs/providers
// directory so we know the shipped YAML plus the registered factories
// produce a non-empty manager on startup.
func TestManagerLoadsBundledConfigs(t *testing.T) {
	t.Parallel()
	// The tests run from each package's own directory; repo root is
	// three levels up from internal/provider.
	dir := filepath.Join("..", "..", "configs", "providers")
	m := provider.NewManager()
	if err := m.LoadDir(dir); err != nil {
		t.Fatalf("load: %v", err)
	}
	ids := m.IDs()
	if len(ids) == 0 {
		t.Fatal("expected at least one provider loaded")
	}
	if p, ok := m.Lookup("volc"); !ok || p.ID() != "volc" {
		t.Fatalf("volc provider not loaded")
	}
}
