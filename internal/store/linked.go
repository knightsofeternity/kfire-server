package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// LinkedAccount is an external platform account linked to a user (Steam, …).
type LinkedAccount struct {
	Provider       string
	ProviderUserID string
	DisplayName    *string
	AvatarURL      *string
	ProfileURL     *string
	CreatedAt      time.Time
}

// UpsertLinkedAccount links (or refreshes) an external account. One account per
// (user, provider): re-linking updates the stored profile.
func (s *Store) UpsertLinkedAccount(ctx context.Context, userID string, a LinkedAccount) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO linked_accounts
			(user_id, provider, provider_user_id, display_name, avatar_url, profile_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			provider_user_id = EXCLUDED.provider_user_id,
			display_name     = EXCLUDED.display_name,
			avatar_url       = EXCLUDED.avatar_url,
			profile_url      = EXCLUDED.profile_url,
			updated_at       = now()`,
		userID, a.Provider, a.ProviderUserID, a.DisplayName, a.AvatarURL, a.ProfileURL)
	return err
}

// ListLinkedAccounts returns a user's linked external accounts.
func (s *Store) ListLinkedAccounts(ctx context.Context, userID string) ([]LinkedAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider, provider_user_id, display_name, avatar_url, profile_url, created_at
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
			&a.AvatarURL, &a.ProfileURL, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
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
// linked to some other user — used to reject account hijacking.
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
