// Package repo holds postgres-backed access for the after-management
// admin identity (adminauth.admins / adminauth.sessions). Distinct
// from iam/repo on purpose: the operator account model has different
// invariants (no wallet, no api keys, internal staff only) than the
// end-user iam model.
package repo

import "github.com/jackc/pgx/v5/pgxpool"

// Repo wraps a pgx pool so the package's helpers can share one
// connection-pool reference instead of taking it every call.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo bound to the given pool. The pool's lifecycle
// is owned by the caller (typically main.go).
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }
