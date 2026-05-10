package catalog

import (
	"context"
	"sync"
	"time"

	"github.com/llmhub/llmhub/internal/catalog/repo"
)

// skuCache is a tiny TTL cache for the SKU resolver. SKUs are touched
// once per chat call; even a 30s TTL eliminates almost all DB reads
// during burst traffic.
type skuEntry struct {
	sku *repo.SKU
	at  time.Time
	err error
}

var (
	skuMu  sync.RWMutex
	skuMap = make(map[string]skuEntry)
)

const skuCacheTTL = 30 * time.Second

// LookupSKU resolves a model name (which is the SKU id in v0.2) to its
// routing + pricing snapshot. Callers should check ms.Status before
// honoring the result.
func (s *Service) LookupSKU(ctx context.Context, id string) (*repo.SKU, error) {
	if id == "" {
		return nil, repo.ErrNotFound
	}
	skuMu.RLock()
	e, ok := skuMap[id]
	skuMu.RUnlock()
	if ok && time.Since(e.at) < skuCacheTTL {
		return e.sku, e.err
	}
	got, err := s.repo.GetSKU(ctx, id)
	skuMu.Lock()
	skuMap[id] = skuEntry{sku: got, err: err, at: time.Now()}
	skuMu.Unlock()
	return got, err
}

// InvalidateSKUs drops the SKU resolver cache. Call from admin SKU
// CRUD handlers when a SKU's routing or pricing changes.
func InvalidateSKUs() {
	skuMu.Lock()
	defer skuMu.Unlock()
	skuMap = make(map[string]skuEntry)
}
