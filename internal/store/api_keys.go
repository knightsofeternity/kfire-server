package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// APIKey is a read-only public-API credential. The cleartext key is never
// stored; only its SHA-256 hash and a short display prefix.
type APIKey struct {
	ID         string
	Label      string
	KeyPrefix  string
	CreatedBy  *string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// CreateAPIKey inserts a new key and returns its generated id.
func (s *Store) CreateAPIKey(ctx context.Context, label, keyPrefix string, keyHash []byte, createdBy string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO api_keys (label, key_prefix, key_hash, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		label, keyPrefix, keyHash, createdBy).Scan(&id)
	return id, err
}

// LookupAPIKey returns the non-revoked key matching the given hash, or
// ErrNotFound. Used by the auth middleware on every public request.
func (s *Store) LookupAPIKey(ctx context.Context, keyHash []byte) (APIKey, error) {
	var k APIKey
	err := s.pool.QueryRow(ctx, `
		SELECT id, label, key_prefix, created_by, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1 AND revoked_at IS NULL`, keyHash).
		Scan(&k.ID, &k.Label, &k.KeyPrefix, &k.CreatedBy, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrNotFound
	}
	return k, err
}

// ListAPIKeys returns all keys (including revoked, for audit), newest first.
func (s *Store) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, label, key_prefix, created_by, created_at, last_used_at, revoked_at
		FROM api_keys
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Label, &k.KeyPrefix, &k.CreatedBy,
			&k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeAPIKey marks a key revoked. Returns ErrNotFound if no such (non-revoked)
// key exists, so the caller can 404; revoking an already-revoked key is a no-op
// that returns ErrNotFound.
func (s *Store) RevokeAPIKey(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchAPIKeyLastUsed updates last_used_at at most once per minute, so a busy
// key doesn't cause a write on every request.
func (s *Store) TouchAPIKeyLastUsed(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET last_used_at = now()
		WHERE id = $1
		  AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, id)
	return err
}
