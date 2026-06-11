package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// LinkedAccount is an external platform account linked to a user (Steam, …).
// OAuth tokens, when present, are already AES-256-GCM encrypted.
type LinkedAccount struct {
	Provider        string
	ProviderUserID  string
	DisplayName     *string
	AvatarURL       *string
	ProfileURL      *string
	AccessTokenEnc  []byte
	RefreshTokenEnc []byte
	TokenExpiresAt  *time.Time
	Scopes          []string
	CreatedAt       time.Time
}

// UpsertLinkedAccount links (or refreshes) an external account. One account per
// (user, provider): re-linking updates the stored profile and tokens.
func (s *Store) UpsertLinkedAccount(ctx context.Context, userID string, a LinkedAccount) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO linked_accounts
			(user_id, provider, provider_user_id, display_name, avatar_url, profile_url,
			 access_token_enc, refresh_token_enc, token_expires_at, scopes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			provider_user_id  = EXCLUDED.provider_user_id,
			display_name      = EXCLUDED.display_name,
			avatar_url        = EXCLUDED.avatar_url,
			profile_url       = EXCLUDED.profile_url,
			access_token_enc  = EXCLUDED.access_token_enc,
			refresh_token_enc = EXCLUDED.refresh_token_enc,
			token_expires_at  = EXCLUDED.token_expires_at,
			scopes            = EXCLUDED.scopes,
			updated_at        = now()`,
		userID, a.Provider, a.ProviderUserID, a.DisplayName, a.AvatarURL, a.ProfileURL,
		a.AccessTokenEnc, a.RefreshTokenEnc, a.TokenExpiresAt, a.Scopes)
	return err
}

// ListLinkedAccounts returns a user's linked external accounts.
func (s *Store) ListLinkedAccounts(ctx context.Context, userID string) ([]LinkedAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider, provider_user_id, display_name, avatar_url, profile_url, scopes, created_at
		FROM linked_accounts WHERE user_id = $1
		ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LinkedAccount
	for rows.Next() {
		var a LinkedAccount
		if err := rows.Scan(&a.Provider, &a.ProviderUserID, &a.DisplayName,
			&a.AvatarURL, &a.ProfileURL, &a.Scopes, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetLinkedAccount returns one provider link for a user, or ErrNotFound.
func (s *Store) GetLinkedAccount(ctx context.Context, userID, provider string) (LinkedAccount, error) {
	var a LinkedAccount
	err := s.pool.QueryRow(ctx, `
		SELECT provider, provider_user_id, display_name, avatar_url, profile_url, created_at
		FROM linked_accounts WHERE user_id = $1 AND provider = $2`, userID, provider).
		Scan(&a.Provider, &a.ProviderUserID, &a.DisplayName, &a.AvatarURL, &a.ProfileURL, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return LinkedAccount{}, ErrNotFound
	}
	return a, err
}

// DeleteLinkedAccount unlinks a provider from a user. Returns ErrNotFound when
// nothing was linked.
func (s *Store) DeleteLinkedAccount(ctx context.Context, userID, provider string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM linked_accounts WHERE user_id = $1 AND provider = $2`, userID, provider)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ProviderLinked reports whether a (provider, provider_user_id) is already
// linked to some other user - used to reject account hijacking.
func (s *Store) ProviderLinkedToOther(ctx context.Context, provider, providerUserID, userID string) (bool, error) {
	var other string
	err := s.pool.QueryRow(ctx, `
		SELECT user_id FROM linked_accounts
		WHERE provider = $1 AND provider_user_id = $2`, provider, providerUserID).Scan(&other)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return other != userID, nil
}

// LinkedToken is a member's stored OAuth token and granted scopes for a provider.
type LinkedToken struct {
	ProviderUserID string
	DisplayName    *string
	AccessTokenEnc []byte
	TokenExpiresAt *time.Time
	Scopes         []string
}

// GetLinkedToken returns the stored token material for (user, provider).
func (s *Store) GetLinkedToken(ctx context.Context, userID, provider string) (LinkedToken, error) {
	var t LinkedToken
	err := s.pool.QueryRow(ctx, `
		SELECT provider_user_id, display_name, access_token_enc, token_expires_at, scopes
		FROM linked_accounts WHERE user_id = $1 AND provider = $2`,
		userID, provider).
		Scan(&t.ProviderUserID, &t.DisplayName, &t.AccessTokenEnc, &t.TokenExpiresAt, &t.Scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}
