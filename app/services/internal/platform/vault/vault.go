// Package vault is the abstraction over HashiCorp Vault (or other
// secret stores) for fetching upstream provider credentials.
//
// The database never stores credentials directly; instead each
// pool.api_keys row carries a vault_ref (e.g.
// "vault://secret/pool/123/key/45"). At call time the gateway resolves
// the ref through this package with a short-lived in-memory cache.
package vault

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// Resolver looks up a secret by a vault reference string.
// The returned map follows a small convention:
//
//	"api_key"    -> bearer token
//	"access_key" -> AK for ak/sk auth
//	"secret_key" -> SK for ak/sk auth
//	"session_token" -> STS-style temporary token
type Resolver interface {
	Resolve(ctx context.Context, ref string) (map[string]string, error)
}

// ErrNotFound is returned when the given ref does not exist.
var ErrNotFound = errors.New("vault: secret not found")

// Stub is a no-op resolver used in tests and during early bootstrap.
type Stub struct{}

// Resolve always returns ErrNotFound.
func (Stub) Resolve(_ context.Context, _ string) (map[string]string, error) {
	return nil, ErrNotFound
}

// DevInline is a development resolver: it treats the ref itself as the
// secret material, so operators can paste a real upstream API key
// directly into the pool.api_keys.vault_ref column for local testing.
// Supported forms:
//
//	"devkey://<bearer>"     → returns {"api_key": "<bearer>"}
//	"devaksk://<ak>:<sk>"   → returns {"access_key": "<ak>", "secret_key": "<sk>"}
//
// Never enable this resolver in production.
type DevInline struct{}

// Resolve parses dev:// URLs into credential maps.
func (DevInline) Resolve(_ context.Context, ref string) (map[string]string, error) {
	switch {
	case strings.HasPrefix(ref, "devkey://"):
		key := strings.TrimPrefix(ref, "devkey://")
		if key == "" {
			return nil, ErrNotFound
		}
		return map[string]string{"api_key": key}, nil
	case strings.HasPrefix(ref, "devaksk://"):
		rest := strings.TrimPrefix(ref, "devaksk://")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return nil, ErrNotFound
		}
		return map[string]string{"access_key": parts[0], "secret_key": parts[1]}, nil
	}
	return nil, ErrNotFound
}

// Cached wraps any Resolver with a short in-process TTL cache. Safe to
// share across goroutines. Cache misses fall through to the underlying
// resolver.
type Cached struct {
	Upstream Resolver
	TTL      time.Duration

	mu   sync.RWMutex
	data map[string]cacheEntry
}

type cacheEntry struct {
	secret map[string]string
	at     time.Time
}

// NewCached constructs a Cached resolver with a sensible default TTL.
func NewCached(up Resolver) *Cached {
	return &Cached{Upstream: up, TTL: 60 * time.Second, data: make(map[string]cacheEntry)}
}

// Resolve satisfies Resolver.
func (c *Cached) Resolve(ctx context.Context, ref string) (map[string]string, error) {
	c.mu.RLock()
	e, ok := c.data[ref]
	c.mu.RUnlock()
	if ok && time.Since(e.at) < c.TTL {
		return cloneMap(e.secret), nil
	}
	sec, err := c.Upstream.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.data[ref] = cacheEntry{secret: cloneMap(sec), at: time.Now()}
	c.mu.Unlock()
	return sec, nil
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
