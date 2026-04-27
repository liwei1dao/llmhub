// Package ratelimit implements a small fixed-bucket sliding-window
// counter used by the gateway to cap per-user and per-api-key QPS.
//
// Two backends are provided:
//
//   - Redis: shared, survives gateway restarts; preferred in production.
//   - Memory: in-process map, used in dev and as a fallback when Redis
//     is unavailable.
//
// The algorithm is intentionally simple (one counter per second bucket)
// so it composes easily with other middleware. Callers tolerate a small
// steady-state burst; exact fairness lives in M7.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter decides whether a caller identified by key may proceed given
// a per-second quota.
type Limiter interface {
	Allow(ctx context.Context, key string, perSecond int) (bool, error)
}

// ----- Memory backend -----

type memBucket struct {
	start time.Time
	count int
}

// Memory is an in-process limiter. Safe across goroutines.
type Memory struct {
	mu    sync.Mutex
	data  map[string]*memBucket
}

// NewMemory returns an empty Memory limiter.
func NewMemory() *Memory { return &Memory{data: make(map[string]*memBucket)} }

// Allow returns true if the caller's second-granularity window has
// capacity left, otherwise false.
func (m *Memory) Allow(_ context.Context, key string, perSecond int) (bool, error) {
	if perSecond <= 0 {
		return true, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	b, ok := m.data[key]
	if !ok || now.Sub(b.start) >= time.Second {
		m.data[key] = &memBucket{start: now, count: 1}
		return true, nil
	}
	if b.count >= perSecond {
		return false, nil
	}
	b.count++
	return true, nil
}

// ----- Redis backend -----

// Redis is a limiter backed by INCR on second-granularity keys. Keys
// expire after two seconds to let the next window start clean.
type Redis struct {
	Client *redis.Client
	Prefix string // typically "llmhub:rl:"
}

// NewRedis returns a Redis-backed limiter.
func NewRedis(c *redis.Client, prefix string) *Redis {
	if prefix == "" {
		prefix = "llmhub:rl:"
	}
	return &Redis{Client: c, Prefix: prefix}
}

// Allow implements Limiter against Redis.
func (r *Redis) Allow(ctx context.Context, key string, perSecond int) (bool, error) {
	if perSecond <= 0 {
		return true, nil
	}
	bucket := time.Now().Unix()
	fullKey := r.Prefix + key + ":" + time.Unix(bucket, 0).Format("20060102150405")

	pipe := r.Client.Pipeline()
	incr := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, 2*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return incr.Val() <= int64(perSecond), nil
}

// ----- Fallback composition -----

// Fallback wraps a primary limiter and uses a secondary one if the
// primary errors (e.g. Redis unavailable). This lets the gateway stay
// available even when shared state is temporarily down.
type Fallback struct {
	Primary   Limiter
	Secondary Limiter
}

// Allow tries the primary limiter first and falls back to secondary
// on error. Primary-denied is never overridden.
func (f Fallback) Allow(ctx context.Context, key string, perSecond int) (bool, error) {
	ok, err := f.Primary.Allow(ctx, key, perSecond)
	if err == nil {
		return ok, nil
	}
	if f.Secondary == nil {
		return true, err
	}
	return f.Secondary.Allow(ctx, key, perSecond)
}
