package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when an admin / session is missing.
var ErrNotFound = errors.New("adminauth/repo: not found")

// Admin is the persistent representation of a back-office operator.
type Admin struct {
	ID            int64
	Account       string
	PasswordHash  string
	DisplayName   *string
	Status        string // active / disabled
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastLoginAt   *time.Time
}

const insertAdminSQL = `
INSERT INTO adminauth.admins (account, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING id, status, created_at, updated_at
`

// CreateAdmin inserts a new admin and returns the hydrated row.
func (r *Repo) CreateAdmin(ctx context.Context, account, passwordHash, displayName string) (*Admin, error) {
	a := &Admin{Account: account, PasswordHash: passwordHash}
	if displayName != "" {
		a.DisplayName = &displayName
	}
	err := r.pool.QueryRow(ctx, insertAdminSQL, account, passwordHash, nilIfEmpty(displayName)).Scan(
		&a.ID, &a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

const selectAdminByAccountSQL = `
SELECT id, account, password_hash, display_name, status, created_at, updated_at, last_login_at
FROM adminauth.admins WHERE account = $1
`

// GetAdminByAccount loads an admin by login account.
func (r *Repo) GetAdminByAccount(ctx context.Context, account string) (*Admin, error) {
	var a Admin
	err := r.pool.QueryRow(ctx, selectAdminByAccountSQL, account).Scan(
		&a.ID, &a.Account, &a.PasswordHash, &a.DisplayName, &a.Status,
		&a.CreatedAt, &a.UpdatedAt, &a.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

const selectAdminByIDSQL = `
SELECT id, account, password_hash, display_name, status, created_at, updated_at, last_login_at
FROM adminauth.admins WHERE id = $1
`

// GetAdminByID loads an admin by primary key.
func (r *Repo) GetAdminByID(ctx context.Context, id int64) (*Admin, error) {
	var a Admin
	err := r.pool.QueryRow(ctx, selectAdminByIDSQL, id).Scan(
		&a.ID, &a.Account, &a.PasswordHash, &a.DisplayName, &a.Status,
		&a.CreatedAt, &a.UpdatedAt, &a.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CountAdmins returns the total number of admins regardless of status.
// Used by bootstrap to detect a fresh install.
func (r *Repo) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM adminauth.admins`).Scan(&n)
	return n, err
}

// TouchLastLogin updates last_login_at to now().
func (r *Repo) TouchLastLogin(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE adminauth.admins SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
