// Package capability defines the core abstractions for AI capability
// domains (chat, embedding, asr, tts, translate, image, ...).
//
// Each capability package under internal/capability/<id>/ implements the
// generic Capability[Req, Resp] interface for its own Req/Resp types and
// registers itself via init() so gateway routing can look it up by id.
package capability

import (
	"context"
	"sync"

	"github.com/llmhub/llmhub/internal/domain"
)

// Capability is the generic contract a capability domain must implement.
// Req and Resp are the capability-specific request/response types.
type Capability[Req any, Resp any] interface {
	ID() string
	BillingUnit() domain.BillingUnit
	Estimate(ctx context.Context, req Req) (domain.BillingEstimate, error)
	RequiredTransport() []domain.Transport
}

// Descriptor is the type-erased view used by the registry so heterogeneous
// capabilities can coexist in one lookup table without Go generics
// complicating the registry itself.
type Descriptor struct {
	ID          string
	Unit        domain.BillingUnit
	Transports  []domain.Transport
	Description string
}

// Registry is the global capability descriptor table. Concrete typed
// implementations are registered from their own packages.
var (
	registryMu   sync.RWMutex
	descriptors  = map[string]Descriptor{}
)

// Register installs a capability descriptor. Must be called from init().
func Register(d Descriptor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := descriptors[d.ID]; ok {
		panic("capability already registered: " + d.ID)
	}
	descriptors[d.ID] = d
}

// Get fetches a descriptor by id.
func Get(id string) (Descriptor, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	d, ok := descriptors[id]
	return d, ok
}

// All returns a copy of every registered descriptor.
func All() []Descriptor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Descriptor, 0, len(descriptors))
	for _, d := range descriptors {
		out = append(out, d)
	}
	return out
}
