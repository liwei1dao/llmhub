package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a queried row does not exist.
var ErrNotFound = errors.New("iam/repo: not found")

// User is the persistent representation of an IAM user.
type User struct {
	ID                 int64
	Email              *string
	Phone              *string
	PasswordHash       string
	DisplayName        *string
	Status             string
	RealnameLevel      int16
	RiskScore          int16
	QPSLimit           int32
	DailySpendLimitCents int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastLoginAt        *time.Time
}

const insertUserSQL = `
INSERT INTO iam.users (email, phone, password_hash, display_name)
VALUES ($1, $2, $3, $4)
RETURNING id, status, risk_score, qps_limit, daily_spend_limit_cents, created_at, updated_at
`

// CreateUser inserts a user and returns the hydrated row.
// email or phone (or both) must be non-nil.
func (r *Repo) CreateUser(ctx context.Context, email, phone *string, passwordHash, displayName string) (*User, error) {
	u := &User{
		Email:        email,
		Phone:        phone,
		PasswordHash: passwordHash,
		DisplayName:  &displayName,
	}
	err := r.pool.QueryRow(ctx, insertUserSQL, email, phone, passwordHash, displayName).Scan(
		&u.ID, &u.Status, &u.RiskScore, &u.QPSLimit, &u.DailySpendLimitCents,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

const selectUserByIDSQL = `
SELECT id, email, phone, password_hash, display_name, status,
       realname_level, risk_score, qps_limit, daily_spend_limit_cents,
       created_at, updated_at, last_login_at
FROM iam.users WHERE id = $1
`

// GetUserByID loads a user by primary key.
func (r *Repo) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, selectUserByIDSQL, id).Scan(
		&u.ID, &u.Email, &u.Phone, &u.PasswordHash, &u.DisplayName, &u.Status,
		&u.RealnameLevel, &u.RiskScore, &u.QPSLimit, &u.DailySpendLimitCents,
		&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

const selectUserByEmailSQL = `
SELECT id, email, phone, password_hash, display_name, status,
       realname_level, risk_score, qps_limit, daily_spend_limit_cents,
       created_at, updated_at, last_login_at
FROM iam.users WHERE email = $1
`

// GetUserByEmail loads a user by email.
func (r *Repo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, selectUserByEmailSQL, email).Scan(
		&u.ID, &u.Email, &u.Phone, &u.PasswordHash, &u.DisplayName, &u.Status,
		&u.RealnameLevel, &u.RiskScore, &u.QPSLimit, &u.DailySpendLimitCents,
		&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// TouchLastLogin updates last_login_at to now().
func (r *Repo) TouchLastLogin(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE iam.users SET last_login_at = NOW() WHERE id = $1`, userID)
	return err
}
