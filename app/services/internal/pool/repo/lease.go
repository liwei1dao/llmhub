package repo

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
)

// Lease is one row in pool.leases — the SDK-facing receipt that says
// "you, user X, are allowed to use binding Y for SKU Z until T". The
// binding's real auth_payload is resolved separately via vault using
// pool.credentials.auth_payload_ref; we don't store the secret here.
type Lease struct {
	ID                int64
	LeaseID           string // UUID, public-facing
	UserID            int64
	APIKeyID          int64
	SKUID             string
	BindingID         int64
	CredentialID      int64
	ClientFingerprint string
	ClientIP          *net.IP
	Status            string
	IssuedAt          time.Time
	ExpiresAt         time.Time
	RevokedAt         *time.Time
	RevokeReason      string
	LastUsedAt        *time.Time
	UseCount          int32
	TotalInputUnits   int64
	TotalOutputUnits  int64
}

// LeaseRepo is the data-access surface for pool.leases.
type LeaseRepo struct{ r *Repo }

// Leases is sugar for repo.NewLeaseRepo.
func (r *Repo) Leases() *LeaseRepo { return &LeaseRepo{r: r} }

// Create writes a new lease row. Caller must have already pick-ed the
// binding and computed expires_at; this method just persists.
func (lr *LeaseRepo) Create(ctx context.Context, l *Lease) (string, error) {
	const sql = `
INSERT INTO pool.leases
       (user_id, api_key_id, sku_id, binding_id, credential_id,
        client_fingerprint, client_ip, expires_at)
VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), $7, $8)
RETURNING id, lease_id, status, issued_at
`
	var ip any
	if l.ClientIP != nil {
		ip = l.ClientIP.String()
	}
	err := lr.r.pool.QueryRow(ctx, sql,
		l.UserID, l.APIKeyID, l.SKUID, l.BindingID, l.CredentialID,
		l.ClientFingerprint, ip, l.ExpiresAt,
	).Scan(&l.ID, &l.LeaseID, &l.Status, &l.IssuedAt)
	return l.LeaseID, err
}

// GetActive resolves a public lease_id (UUID) to the lease row, only
// if it's still active and not expired. Used by /sdk/usage/report.
func (lr *LeaseRepo) GetActive(ctx context.Context, leaseID string) (*Lease, error) {
	const sql = `
SELECT id, lease_id::text, user_id, api_key_id, sku_id, binding_id, credential_id,
       COALESCE(client_fingerprint,''), client_ip::text, status,
       issued_at, expires_at, revoked_at, COALESCE(revoke_reason,''),
       last_used_at, use_count, total_input_units, total_output_units
FROM pool.leases
WHERE lease_id = $1::uuid AND status = 'active' AND expires_at > NOW()
`
	var ipStr *string
	var l Lease
	err := lr.r.pool.QueryRow(ctx, sql, leaseID).Scan(
		&l.ID, &l.LeaseID, &l.UserID, &l.APIKeyID, &l.SKUID,
		&l.BindingID, &l.CredentialID,
		&l.ClientFingerprint, &ipStr, &l.Status,
		&l.IssuedAt, &l.ExpiresAt, &l.RevokedAt, &l.RevokeReason,
		&l.LastUsedAt, &l.UseCount, &l.TotalInputUnits, &l.TotalOutputUnits,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ipStr != nil && *ipStr != "" {
		ip := net.ParseIP(*ipStr)
		l.ClientIP = &ip
	}
	return &l, nil
}

// AddUsage atomically increments use_count + token totals on a lease.
// Caller calls this from /sdk/usage/report.
func (lr *LeaseRepo) AddUsage(ctx context.Context, leaseID string, inputUnits, outputUnits int64) error {
	const sql = `
UPDATE pool.leases
SET use_count = use_count + 1,
    total_input_units = total_input_units + $1,
    total_output_units = total_output_units + $2,
    last_used_at = NOW()
WHERE lease_id = $3::uuid AND status = 'active'
`
	tag, err := lr.r.pool.Exec(ctx, sql, inputUnits, outputUnits, leaseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Revoke flips a lease to revoked status. Used by admin rotation /
// risk-control auto-revoke / period-end cleanup.
func (lr *LeaseRepo) Revoke(ctx context.Context, leaseID, reason string) error {
	const sql = `
UPDATE pool.leases
SET status        = 'revoked',
    revoked_at    = NOW(),
    revoke_reason = NULLIF($2,'')
WHERE lease_id = $1::uuid AND status = 'active'
`
	tag, err := lr.r.pool.Exec(ctx, sql, leaseID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LeaseFilter narrows the admin lease-listing query. Empty fields
// are ignored; OnlyActive is the most common admin view.
type LeaseFilter struct {
	UserID     int64
	SKUID      string
	BindingID  int64
	Status     string // active / revoked / expired; empty = all
	OnlyActive bool   // shortcut: status='active' AND expires_at > NOW()
	Limit      int
}

// List returns leases matching the filter, newest issued first. Used
// by the admin /api/admin/leases page so operators can spot anomalies
// (one user holding hundreds of leases, a binding's leases all hitting
// the same SKU) and revoke individual entries.
func (lr *LeaseRepo) List(ctx context.Context, f LeaseFilter) ([]Lease, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	// Build a small WHERE clause inline. Each predicate is a constant
	// SQL fragment + a bound param; nothing user-controlled goes into
	// the SQL string.
	where := []string{"1 = 1"}
	args := []any{}
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, clause+itoa(len(args)))
	}
	if f.UserID > 0 {
		add("user_id = $", f.UserID)
	}
	if f.SKUID != "" {
		add("sku_id = $", f.SKUID)
	}
	if f.BindingID > 0 {
		add("binding_id = $", f.BindingID)
	}
	if f.OnlyActive {
		where = append(where, "status = 'active'", "expires_at > NOW()")
	} else if f.Status != "" {
		add("status = $", f.Status)
	}
	args = append(args, f.Limit)
	limitArg := "$" + itoa(len(args))

	sql := `
SELECT id, lease_id::text, user_id, api_key_id, sku_id, binding_id, credential_id,
       COALESCE(client_fingerprint,''), client_ip::text, status,
       issued_at, expires_at, revoked_at, COALESCE(revoke_reason,''),
       last_used_at, use_count, total_input_units, total_output_units
FROM pool.leases
WHERE ` + joinAnd(where) + `
ORDER BY issued_at DESC
LIMIT ` + limitArg

	rows, err := lr.r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lease
	for rows.Next() {
		var l Lease
		var ipStr *string
		if err := rows.Scan(
			&l.ID, &l.LeaseID, &l.UserID, &l.APIKeyID, &l.SKUID,
			&l.BindingID, &l.CredentialID,
			&l.ClientFingerprint, &ipStr, &l.Status,
			&l.IssuedAt, &l.ExpiresAt, &l.RevokedAt, &l.RevokeReason,
			&l.LastUsedAt, &l.UseCount, &l.TotalInputUnits, &l.TotalOutputUnits,
		); err != nil {
			return nil, err
		}
		if ipStr != nil && *ipStr != "" {
			ip := net.ParseIP(*ipStr)
			l.ClientIP = &ip
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// itoa / joinAnd are tiny string helpers kept local so this package
// doesn't pull strconv / strings just for two SQL builders.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

// SweepExpired marks expired-but-still-active leases as 'expired'.
// Run from a background worker every minute or so.
func (lr *LeaseRepo) SweepExpired(ctx context.Context, batch int) (int64, error) {
	if batch <= 0 {
		batch = 500
	}
	const sql = `
UPDATE pool.leases
SET status = 'expired'
WHERE id IN (
  SELECT id FROM pool.leases
  WHERE status = 'active' AND expires_at <= NOW()
  LIMIT $1
)
`
	tag, err := lr.r.pool.Exec(ctx, sql, batch)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
