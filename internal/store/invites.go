package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Invite is a shareable, single-use registration token.
type Invite struct {
	Code      string
	Note      *string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateInvite stores a new pending invite. createdBy may be nil (e.g. an invite
// minted by an API key whose creating admin was deleted), stored as NULL.
func (s *Store) CreateInvite(ctx context.Context, code, note, role string, createdBy *string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO invites (code, note, role, created_by, expires_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5)`,
		code, note, role, createdBy, expiresAt)
	return err
}

// PeekInvite returns a pending (unused, unexpired) invite's role, or
// ErrNotFound. It does not consume the invite.
func (s *Store) PeekInvite(ctx context.Context, code string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT role FROM invites
		WHERE code = $1 AND used_at IS NULL AND expires_at > now()`, code).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

// MarkInviteUsed atomically consumes a pending invite. Returns ErrNotFound if
// it was already used or expired (e.g. a race), so the caller can react.
func (s *Store) MarkInviteUsed(ctx context.Context, code, usedBy string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE invites SET used_at = now(), used_by = $2
		WHERE code = $1 AND used_at IS NULL AND expires_at > now()`, code, usedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPendingInvites returns unused, unexpired invites, newest first.
func (s *Store) ListPendingInvites(ctx context.Context) ([]Invite, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT code, note, role, created_at, expires_at
		FROM invites
		WHERE used_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Invite
	for rows.Next() {
		var i Invite
		if err := rows.Scan(&i.Code, &i.Note, &i.Role, &i.CreatedAt, &i.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// DeleteInvite revokes a pending invite. Returns ErrNotFound when absent.
func (s *Store) DeleteInvite(ctx context.Context, code string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM invites WHERE code = $1`, code)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
