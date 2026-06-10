package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreatePasswordReset stores a reset token hash for a user, replacing any
// pending one so only the latest link works.
func (s *Store) CreatePasswordReset(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, userID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO password_resets (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt)
	return err
}

// PeekPasswordReset returns the target user's id and username when the token is
// valid and unexpired, without consuming it (so the reset page can confirm
// whose account it is).
func (s *Store) PeekPasswordReset(ctx context.Context, tokenHash []byte) (userID, username string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT u.id, u.username
		FROM password_resets pr
		JOIN users u ON u.id = pr.user_id
		WHERE pr.token_hash = $1 AND pr.expires_at > now()`, tokenHash).
		Scan(&userID, &username)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return userID, username, err
}

// ConsumePasswordReset validates and deletes the token (single use), returning
// the user it belongs to.
func (s *Store) ConsumePasswordReset(ctx context.Context, tokenHash []byte) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx, `
		DELETE FROM password_resets
		WHERE token_hash = $1 AND expires_at > now()
		RETURNING user_id`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}
