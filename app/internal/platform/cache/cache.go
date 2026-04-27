// Package cache provides a Redis client used for scheduler hot paths,
// rate limiting, session stickiness, and API key lookup caches.
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/llmhub/llmhub/internal/platform/config"
)

// Open returns a connected Redis client.
func Open(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis.addr is required")
	}
	c := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return c, nil
}
