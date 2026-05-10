package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// VendorAccount mirrors a row in pool.vendor_accounts. It is the
// "行政/结算单元" — used to query upstream balance/billing endpoints,
// never to call business APIs directly.
type VendorAccount struct {
	ID                  int64
	VendorID            string
	Name                string
	Entity              string
	ConsoleURL          string
	MasterAuthRef       string // vault ref, never the secret itself
	LastBalanceCents    *int64
	LastBalanceCurrency string
	LastBalanceAt       *time.Time
	LastBalanceError    string
	Status              string // active / frozen / archived
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// VendorAccountRepo is the data-access surface for pool.vendor_accounts.
//
// Construct via NewVendorAccountRepo(pool). Shares the same *pgxpool.Pool
// as the rest of the pool repo family.
type VendorAccountRepo struct{ r *Repo }

// NewVendorAccountRepo returns a VendorAccountRepo that piggybacks on
// the pool's shared pgx pool.
func NewVendorAccountRepo(r *Repo) *VendorAccountRepo { return &VendorAccountRepo{r: r} }

// VendorAccounts is sugar for pool.NewVendorAccountRepo(repo).
func (r *Repo) VendorAccounts() *VendorAccountRepo { return NewVendorAccountRepo(r) }

// Create inserts a new vendor account. The caller is responsible for
// having already written master_auth_ref to vault — only the ref is
// persisted here.
func (vr *VendorAccountRepo) Create(ctx context.Context, a *VendorAccount) (int64, error) {
	const q = `
INSERT INTO pool.vendor_accounts
       (vendor_id, name, entity, console_url, master_auth_ref, status)
VALUES ($1, $2, $3, NULLIF($4,''), $5, COALESCE(NULLIF($6,''),'active'))
RETURNING id, created_at, updated_at
`
	var id int64
	err := vr.r.pool.QueryRow(ctx, q,
		a.VendorID, a.Name, a.Entity, a.ConsoleURL, a.MasterAuthRef, a.Status,
	).Scan(&id, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return 0, err
	}
	a.ID = id
	return id, nil
}

// Get fetches a single vendor account by id.
func (vr *VendorAccountRepo) Get(ctx context.Context, id int64) (*VendorAccount, error) {
	const q = `
SELECT id, vendor_id, name, COALESCE(entity,''), COALESCE(console_url,''),
       master_auth_ref,
       last_balance_cents, COALESCE(last_balance_currency,''),
       last_balance_at, COALESCE(last_balance_error,''),
       status, created_at, updated_at
FROM pool.vendor_accounts
WHERE id = $1
`
	a := &VendorAccount{}
	err := vr.r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.VendorID, &a.Name, &a.Entity, &a.ConsoleURL,
		&a.MasterAuthRef,
		&a.LastBalanceCents, &a.LastBalanceCurrency,
		&a.LastBalanceAt, &a.LastBalanceError,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// VendorAccountFilter narrows a List() call.
type VendorAccountFilter struct {
	VendorID string
	Status   string
	Search   string // matches name / entity (ILIKE)
	Limit    int
}

// List returns vendor accounts matching the filter, newest first.
// Default Limit is 200; the caller can override but the function caps at 1000.
func (vr *VendorAccountRepo) List(ctx context.Context, f VendorAccountFilter) ([]VendorAccount, error) {
	if f.Limit <= 0 {
		f.Limit = 200
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}
	const q = `
SELECT id, vendor_id, name, COALESCE(entity,''), COALESCE(console_url,''),
       master_auth_ref,
       last_balance_cents, COALESCE(last_balance_currency,''),
       last_balance_at, COALESCE(last_balance_error,''),
       status, created_at, updated_at
FROM pool.vendor_accounts
WHERE ($1 = '' OR vendor_id = $1)
  AND ($2 = '' OR status = $2)
  AND ($3 = '' OR name ILIKE '%'||$3||'%' OR COALESCE(entity,'') ILIKE '%'||$3||'%')
ORDER BY created_at DESC
LIMIT $4
`
	rows, err := vr.r.pool.Query(ctx, q, f.VendorID, f.Status, f.Search, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VendorAccount
	for rows.Next() {
		var a VendorAccount
		if err := rows.Scan(
			&a.ID, &a.VendorID, &a.Name, &a.Entity, &a.ConsoleURL,
			&a.MasterAuthRef,
			&a.LastBalanceCents, &a.LastBalanceCurrency,
			&a.LastBalanceAt, &a.LastBalanceError,
			&a.Status, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateBalance records a fresh balance snapshot. Pass cents=nil + err
// to record a failed query attempt.
func (vr *VendorAccountRepo) UpdateBalance(ctx context.Context, id int64, cents *int64, currency, errMsg string) error {
	const q = `
UPDATE pool.vendor_accounts
SET last_balance_cents    = $1,
    last_balance_currency = NULLIF($2,''),
    last_balance_at       = NOW(),
    last_balance_error    = NULLIF($3,''),
    updated_at            = NOW()
WHERE id = $4
`
	tag, err := vr.r.pool.Exec(ctx, q, cents, currency, errMsg, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus flips the account's status (active / frozen / archived).
// Archiving cascades to credentials/bindings via application logic in
// the service layer — this method only writes the row.
func (vr *VendorAccountRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	const q = `UPDATE pool.vendor_accounts SET status=$1, updated_at=NOW() WHERE id=$2`
	tag, err := vr.r.pool.Exec(ctx, q, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
