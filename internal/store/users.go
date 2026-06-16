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
	SessionsVisible bool
	BannedAt        *time.Time
	CreatedAt       time.Time
	PresenceStatus  string
}

const userColumns = `id, org_id, username, email, password_hash, role, avatar_url, activity_visible, sessions_visible, banned_at, created_at, presence_status`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.OrgID, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.AvatarURL, &u.ActivityVisible, &u.SessionsVisible, &u.BannedAt, &u.CreatedAt, &u.PresenceStatus)
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

// SetPresenceStatus updates a user's chosen presence status (online, invisible,
// offline). invisible/offline make the member appear offline to other viewers.
func (s *Store) SetPresenceStatus(ctx context.Context, userID, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET presence_status = $2 WHERE id = $1`, userID, status)
	return err
}

// SetSessionsVisible updates a user's recent-sessions privacy toggle.
func (s *Store) SetSessionsVisible(ctx context.Context, userID string, visible bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET sessions_visible = $2 WHERE id = $1`, userID, visible)
	return err
}

// ListUsers returns every account, newest first (admin member management).
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+userColumns+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetUserRole changes a user's role.
func (s *Store) SetUserRole(ctx context.Context, userID, role string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1`, userID, role)
	return err
}

// SetUserBanned bans or unbans a user.
func (s *Store) SetUserBanned(ctx context.Context, userID string, banned bool) error {
	if banned {
		_, err := s.pool.Exec(ctx,
			`UPDATE users SET banned_at = now() WHERE id = $1 AND banned_at IS NULL`, userID)
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE users SET banned_at = NULL WHERE id = $1`, userID)
	return err
}

// CountAdmins returns the number of non-banned admins (to prevent removing the
// last one).
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role = 'admin' AND banned_at IS NULL`).Scan(&n)
	return n, err
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

// SetUserPassword replaces a user's password hash (used by admin-initiated
// password resets).
func (s *Store) SetUserPassword(ctx context.Context, userID, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash)
	return err
}

// UpdateUserAvatarIfEmpty sets the user's avatar only when they don't already
// have one, so adopting a linked Steam avatar never overrides a chosen avatar.
func (s *Store) UpdateUserAvatarIfEmpty(ctx context.Context, userID, avatarURL string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET avatar_url = $2 WHERE id = $1 AND avatar_url IS NULL`,
		userID, avatarURL)
	return err
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
