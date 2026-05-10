package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Credential mirrors a row in pool.credentials. It is the "应用级"
// credential — an upstream business API call uses the auth payload
// referenced here.
type Credential struct {
	ID                   int64
	VendorID             string // denormalized; equals account.vendor_id and product.vendor_id
	AccountID            int64
	ProductID            string
	Name                 string
	Env                  string
	AuthPayloadRef       string // vault ref
	AuthPayloadDigest    string
	Status               string
	HealthScore          int16
	CooldownUntil        *time.Time
	IsolationGroupID     *int64
	LastUsedAt           *time.Time
	LastErrorAt          *time.Time
	ConsecutiveFailures  int32
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CredentialRepo is the data-access surface for pool.credentials.
type CredentialRepo struct{ r *Repo }

// NewCredentialRepo binds to the shared pool.
func NewCredentialRepo(r *Repo) *CredentialRepo { return &CredentialRepo{r: r} }

// Credentials is sugar for repo.NewCredentialRepo.
func (r *Repo) Credentials() *CredentialRepo { return NewCredentialRepo(r) }

// Create inserts a credential. Caller must have validated:
//   - account exists and account.vendor_id == credential.VendorID
//   - product_id is a known catalog.Products id and product.VendorID == VendorID
//   - auth_payload_ref already written to vault
func (cr *CredentialRepo) Create(ctx context.Context, c *Credential) (int64, error) {
	return cr.createTx(ctx, cr.r.pool, c)
}

// createTx is the underlying create; both Create() and the service-level
// "create credential + bindings" transaction route through it.
func (cr *CredentialRepo) createTx(ctx context.Context, q querier, c *Credential) (int64, error) {
	const sql = `
INSERT INTO pool.credentials
       (vendor_id, account_id, product_id, name, env,
        auth_payload_ref, auth_payload_digest, status, isolation_group_id)
VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5,''),'production'),
        $6, NULLIF($7,''), COALESCE(NULLIF($8,''),'active'), $9)
RETURNING id, env, status, health_score, created_at, updated_at
`
	var id int64
	err := q.QueryRow(ctx, sql,
		c.VendorID, c.AccountID, c.ProductID, c.Name, c.Env,
		c.AuthPayloadRef, c.AuthPayloadDigest, c.Status, c.IsolationGroupID,
	).Scan(&id, &c.Env, &c.Status, &c.HealthScore, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return 0, err
	}
	c.ID = id
	return id, nil
}

// Get fetches a credential by id.
func (cr *CredentialRepo) Get(ctx context.Context, id int64) (*Credential, error) {
	const q = `
SELECT id, vendor_id, account_id, product_id, name, env,
       auth_payload_ref, COALESCE(auth_payload_digest,''),
       status, health_score, cooldown_until, isolation_group_id,
       last_used_at, last_error_at, consecutive_failures,
       created_at, updated_at
FROM pool.credentials
WHERE id = $1
`
	c := &Credential{}
	err := cr.r.pool.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.VendorID, &c.AccountID, &c.ProductID, &c.Name, &c.Env,
		&c.AuthPayloadRef, &c.AuthPayloadDigest,
		&c.Status, &c.HealthScore, &c.CooldownUntil, &c.IsolationGroupID,
		&c.LastUsedAt, &c.LastErrorAt, &c.ConsecutiveFailures,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// CredentialFilter narrows List().
type CredentialFilter struct {
	VendorID  string
	ProductID string
	AccountID int64
	Status    string
	Search    string // matches name (ILIKE)
	Limit     int
}

// List returns credentials matching the filter, newest first.
func (cr *CredentialRepo) List(ctx context.Context, f CredentialFilter) ([]Credential, error) {
	if f.Limit <= 0 {
		f.Limit = 200
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}
	const q = `
SELECT id, vendor_id, account_id, product_id, name, env,
       auth_payload_ref, COALESCE(auth_payload_digest,''),
       status, health_score, cooldown_until, isolation_group_id,
       last_used_at, last_error_at, consecutive_failures,
       created_at, updated_at
FROM pool.credentials
WHERE ($1 = '' OR vendor_id = $1)
  AND ($2 = '' OR product_id = $2)
  AND ($3::bigint = 0 OR account_id = $3)
  AND ($4 = '' OR status = $4)
  AND ($5 = '' OR name ILIKE '%'||$5||'%')
ORDER BY created_at DESC
LIMIT $6
`
	rows, err := cr.r.pool.Query(ctx, q,
		f.VendorID, f.ProductID, f.AccountID, f.Status, f.Search, f.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Credential
	for rows.Next() {
		var c Credential
		if err := rows.Scan(
			&c.ID, &c.VendorID, &c.AccountID, &c.ProductID, &c.Name, &c.Env,
			&c.AuthPayloadRef, &c.AuthPayloadDigest,
			&c.Status, &c.HealthScore, &c.CooldownUntil, &c.IsolationGroupID,
			&c.LastUsedAt, &c.LastErrorAt, &c.ConsecutiveFailures,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateStatus flips the credential's status.
func (cr *CredentialRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	const q = `UPDATE pool.credentials SET status=$1, updated_at=NOW() WHERE id=$2`
	tag, err := cr.r.pool.Exec(ctx, q, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AdjustHealth nudges the credential's health score, clamped to [0,100].
// On failure (delta < 0) bumps consecutive_failures and last_error_at.
// Returns the new score.
func (cr *CredentialRepo) AdjustHealth(ctx context.Context, id int64, delta int) (int16, error) {
	const q = `
UPDATE pool.credentials
SET health_score = GREATEST(0, LEAST(100, health_score + $1)),
    consecutive_failures = CASE WHEN $1 < 0 THEN consecutive_failures + 1 ELSE 0 END,
    last_error_at = CASE WHEN $1 < 0 THEN NOW() ELSE last_error_at END,
    last_used_at  = NOW(),
    updated_at    = NOW()
WHERE id = $2
RETURNING health_score
`
	var s int16
	err := cr.r.pool.QueryRow(ctx, q, delta, id).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return s, err
}

// querier is the small subset of pgx interface we need so a Repo
// method can be reused with either *pgxpool.Pool or pgx.Tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
