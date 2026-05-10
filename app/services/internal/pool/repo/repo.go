// Package repo is the data-access layer for the pool domain. The v0.2
// schema is built around five tables:
//
//	pool.vendor_accounts      — master account at an upstream vendor
//	pool.credentials          — credential (API key / AK+SK / ...) under a vendor account
//	pool.credential_services  — schedulable binding: (credential × capability)
//	pool.credential_events    — append-only event log for a credential
//	pool.leases               — SDK lease records (POST /sdk/credentials/issue receipts)
//
// Each table has its own .go file with a small typed repo. Cross-table
// transactions live in transactions.go.
package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is the pool-layer missing-row sentinel.
var ErrNotFound = errors.New("pool/repo: not found")

// Repo is the pool data-access layer.
type Repo struct {
	pool *pgxpool.Pool
}

// New constructs a Repo from a pgx pool.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Pool exposes the underlying pgx pool. Used by handler packages that
// need to run ad-hoc queries against schemas other than pool.* (for
// example the admin SKU page queries catalog.platform_services through
// here).
func (r *Repo) Pool() *pgxpool.Pool { return r.pool }

// withTx runs fn inside a ReadCommitted transaction, rolling back on
// any returned error. Used by the cross-table writes in transactions.go.
func (r *Repo) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
