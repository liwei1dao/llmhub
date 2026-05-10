// Package repo is the data-access layer for the iam domain.
//
// Queries are hand-written against pgx/v5 and exported through a thin
// Repo struct. When we introduce sqlc later the call sites here remain
// stable; the generated code plugs in behind these methods.
package repo

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the IAM data access layer.
type Repo struct {
	pool *pgxpool.Pool
}

// New constructs a Repo from a pgx pool.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }
