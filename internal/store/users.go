package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// User mirrors a row of the users table.
type User struct {
	ID              string
	OrgID           string
	Username        string
	Email           string
	PasswordHash    string
	Role            string
	AvatarURL       *string
	ActivityVisible bool
	BannedAt        *time.Time
	CreatedAt       time.Time
}

const userColumns = `id, org_id, username, email, password_hash, role, avatar_url, activity_visible, banned_at, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.OrgID, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.AvatarURL, &u.ActivityVisible, &u.BannedAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// SetActivityVisible updates a user's presence privacy toggle.
func (s *Store) SetActivityVisible(ctx context.Context, userID string, visible bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET activity_visible = $2 WHERE id = $1`, userID, visible)
	return err
}

// EnsureDefaultOrg returns the instance's organization, creating it on first
// boot (mono-tenant: one server = one org).
func (s *Store) EnsureDefaultOrg(ctx context.Context, name string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM orgs ORDER BY created_at LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx,
			`INSERT INTO orgs (name, slug) VALUES ($1, 'default') RETURNING id`,
			name).Scan(&id)
	}
	return id, err
}

// CountUsers returns the total number of accounts.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser inserts a new account. Returns ErrConflict when the username or
// email is already taken.
func (s *Store) CreateUser(ctx context.Context, orgID, username, email, passwordHash, role string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (org_id, username, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+userColumns,
		orgID, username, email, passwordHash, role)

	u, err := scanUser(row)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return User{}, ErrConflict
	}
	return u, err
}

// GetUserByLogin fetches a user by username or email.
func (s *Store) GetUserByLogin(ctx context.Context, login string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = $1 OR email = $1`, login)
	return scanUser(row)
}

// GetUserByID fetches a user by primary key.
func (s *Store) GetUserByID(ctx context.Context, id string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}
