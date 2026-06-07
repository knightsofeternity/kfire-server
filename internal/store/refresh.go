package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// RefreshToken is the stored (hashed) form of a device-bound refresh token.
type RefreshToken struct {
	UserID    string
	DeviceID  string
	ExpiresAt time.Time
}

// SaveRefreshToken stores (or rotates) the refresh token for a user+device
// pair. One token per device: a new login or refresh on the same device
// replaces the previous token.
func (s *Store) SaveRefreshToken(ctx context.Context, userID, deviceID, deviceName, platform string, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, device_id, device_name, platform, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			token_hash  = EXCLUDED.token_hash,
			-- Rotation passes empty name/platform: keep the stored values.
			device_name = COALESCE(NULLIF(EXCLUDED.device_name, ''), refresh_tokens.device_name),
			platform    = COALESCE(NULLIF(EXCLUDED.platform, ''), refresh_tokens.platform),
			expires_at  = EXCLUDED.expires_at,
			created_at  = now()`,
		userID, deviceID, deviceName, platform, tokenHash, expiresAt)
	return err
}

// GetRefreshToken looks a token up by its SHA-256 hash.
func (s *Store) GetRefreshToken(ctx context.Context, tokenHash []byte) (RefreshToken, error) {
	var rt RefreshToken
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, device_id, expires_at
		FROM refresh_tokens WHERE token_hash = $1`, tokenHash).
		Scan(&rt.UserID, &rt.DeviceID, &rt.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, ErrNotFound
	}
	return rt, err
}

// DeleteRefreshToken revokes the token of a user+device pair (logout).
func (s *Store) DeleteRefreshToken(ctx context.Context, userID, deviceID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1 AND device_id = $2`,
		userID, deviceID)
	return err
}
