// Package repo is the wallet data-access layer. All balance changes
// happen through transactions that bundle an account update with a
// matching row in wallet.transactions for full auditability.
package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo exposes wallet queries.
type Repo struct {
	pool *pgxpool.Pool
}

// New constructs a Repo.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// withTx runs fn inside a serializable-isolation transaction.
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
