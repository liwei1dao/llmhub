// Package db provides a Postgres connection pool (pgx/v5) shared across
// services. Only the pool is exposed; SQL is executed by sqlc-generated
// code in each domain's repo package.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/llmhub/llmhub/internal/platform/config"
)

// Open creates a pgx pool from the given DB config.
func Open(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("db.dsn is required")
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifeMins > 0 {
		poolCfg.MaxConnLifetime = time.Duration(cfg.ConnMaxLifeMins) * time.Minute
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
