package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// PsnBot is the instance's PlayStation bot, as stored. Tokens stay sealed.
type PsnBot struct {
	NPSSOEnc         []byte
	RefreshTokenEnc  []byte
	RefreshExpiresAt *time.Time
	AccessTokenEnc   []byte
	AccessExpiresAt  *time.Time
	OnlineID         *string
	AccountID        *string
	Status           string
	LastError        *string
	NPSSOSetAt       time.Time
	LastOKAt         *time.Time
}

// GetPsnBot returns the bot, or ErrNotFound when no admin has set one.
func (s *Store) GetPsnBot(ctx context.Context) (PsnBot, error) {
	var b PsnBot
	err := s.pool.QueryRow(ctx, `
		SELECT npsso_enc, refresh_token_enc, refresh_expires_at, access_token_enc,
		       access_expires_at, online_id, account_id, status, last_error,
		       npsso_set_at, last_ok_at
		FROM psn_bot WHERE id = 1`).Scan(&b.NPSSOEnc, &b.RefreshTokenEnc, &b.RefreshExpiresAt,
		&b.AccessTokenEnc, &b.AccessExpiresAt, &b.OnlineID, &b.AccountID, &b.Status,
		&b.LastError, &b.NPSSOSetAt, &b.LastOKAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// SetPsnNPSSO stores a new NPSSO and forgets every token derived from the old one.
func (s *Store) SetPsnNPSSO(ctx context.Context, npssoEnc []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO psn_bot (id, npsso_enc, status, npsso_set_at)
		VALUES (1, $1, 'ok', now())
		ON CONFLICT (id) DO UPDATE SET npsso_enc = $1, status = 'ok', last_error = NULL,
		    npsso_set_at = now(), refresh_token_enc = NULL, refresh_expires_at = NULL,
		    access_token_enc = NULL, access_expires_at = NULL`, npssoEnc)
	return err
}

// SavePsnTokens records fresh tokens and the bot's identity, and marks it ok.
func (s *Store) SavePsnTokens(ctx context.Context, accessEnc []byte, accessExp time.Time, refreshEnc []byte, refreshExp time.Time, onlineID, accountID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE psn_bot SET access_token_enc = $1, access_expires_at = $2,
		    refresh_token_enc = $3, refresh_expires_at = $4,
		    online_id = COALESCE(NULLIF($5, ''), online_id),
		    account_id = COALESCE(NULLIF($6, ''), account_id),
		    status = 'ok', last_error = NULL, last_ok_at = now()
		WHERE id = 1`, accessEnc, accessExp, refreshEnc, refreshExp, onlineID, accountID)
	return err
}

// MarkPsnNeedsNPSSO records that Sony refused the NPSSO.
func (s *Store) MarkPsnNeedsNPSSO(ctx context.Context, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE psn_bot SET status = 'needs_npsso', last_error = $1,
		    access_token_enc = NULL, refresh_token_enc = NULL
		WHERE id = 1`, reason)
	return err
}

// psnSlug is the catalog slug of a normalized PlayStation title.
func psnSlug(normalized string) string { return steamSlug(normalized) }

// UpsertPsnGame resolves a PlayStation concept to a catalog game: by concept
// id, then by the slug of its normalized name (adopting the concept, so PS5
// hours join the PC game of the same name), else a new playstation entry. Every title
// id of the concept is mapped to the game.
func (s *Store) UpsertPsnGame(ctx context.Context, conceptID string, titleIDs []string, normalizedName, imageURL string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, icon_url FROM games WHERE psn_concept_id = $1 LIMIT 1`, conceptID).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			UPDATE games SET psn_concept_id = $1
			WHERE id = (SELECT id FROM games WHERE slug = $2 AND psn_concept_id IS NULL LIMIT 1)
			RETURNING id, name, slug, icon_url`, conceptID, psnSlug(normalizedName)).
			Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			INSERT INTO games (name, slug, executable_names, platform, psn_concept_id, icon_url)
			VALUES ($1, $2, '{}', 'playstation', $3, NULLIF($4, ''))
			ON CONFLICT (slug) DO UPDATE SET psn_concept_id = COALESCE(games.psn_concept_id, EXCLUDED.psn_concept_id)
			RETURNING id, name, slug, icon_url`, normalizedName, psnSlug(normalizedName), conceptID, imageURL).
			Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	}
	if err != nil {
		return g, err
	}
	for _, t := range titleIDs {
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO psn_titles (np_title_id, game_id) VALUES ($1, $2)
			ON CONFLICT (np_title_id) DO NOTHING`, t, g.ID); err != nil {
			return g, err
		}
	}
	return g, nil
}

// PsnGameForTitle resolves a title id seen in presence: the mapped game, else
// the catalog game with the normalized name's slug (and the mapping is
// recorded), else a new playstation entry. The concept is unknown here; the next
// library import adopts it.
func (s *Store) PsnGameForTitle(ctx context.Context, titleID, normalizedName string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx, `
		SELECT g.id, g.name, g.slug, g.icon_url FROM psn_titles t JOIN games g ON g.id = t.game_id
		WHERE t.np_title_id = $1`, titleID).Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err == nil {
		return g, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return g, err
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO games (name, slug, executable_names, platform)
		VALUES ($1, $2, '{}', 'playstation')
		ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
		RETURNING id, name, slug, icon_url`, normalizedName, psnSlug(normalizedName)).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err != nil {
		return g, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO psn_titles (np_title_id, game_id) VALUES ($1, $2)
		ON CONFLICT (np_title_id) DO NOTHING`, titleID, g.ID)
	return g, err
}

// PsnLibraryDue reports whether a member's library was never imported or not
// for `every`.
func (s *Store) PsnLibraryDue(ctx context.Context, userID string, every time.Duration) (bool, error) {
	var at time.Time
	err := s.pool.QueryRow(ctx, `SELECT synced_at FROM psn_library_sync WHERE user_id = $1`, userID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return err == nil && time.Since(at) >= every, err
}

// MarkPsnLibrarySynced stamps a member's library import.
func (s *Store) MarkPsnLibrarySynced(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO psn_library_sync (user_id, synced_at) VALUES ($1, now())
		ON CONFLICT (user_id) DO UPDATE SET synced_at = now()`, userID)
	return err
}

// PsnGameForTrophies finds the game a trophy list belongs to, WITHOUT creating
// one: trophy lists are named differently from the store ("World of Tanks"
// against "World of Tanks Modern Armor"), and a new game per trophy list would
// fill the catalog with near-duplicates holding trophies and no hours. The
// list is found by its mapping, else by the slug of its normalized name (and
// the mapping is recorded); ok is false when neither exists.
func (s *Store) PsnGameForTrophies(ctx context.Context, npCommunicationID, normalizedName string) (Game, bool, error) {
	key := "trophy:" + npCommunicationID
	var g Game
	err := s.pool.QueryRow(ctx, `
		SELECT g.id, g.name, g.slug, g.icon_url FROM psn_titles t JOIN games g ON g.id = t.game_id
		WHERE t.np_title_id = $1`, key).Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err == nil {
		return g, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return g, false, err
	}
	err = s.pool.QueryRow(ctx, `SELECT id, name, slug, icon_url FROM games WHERE slug = $1`,
		psnSlug(normalizedName)).Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, false, nil
	}
	if err != nil {
		return g, false, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO psn_titles (np_title_id, game_id) VALUES ($1, $2)
		ON CONFLICT (np_title_id) DO NOTHING`, key, g.ID)
	return g, true, err
}

// DeletePsnBotForTests empties psn_bot. Tests only: nothing in the product
// removes the bot, an admin replaces its NPSSO.
func (s *Store) DeletePsnBotForTests(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM psn_bot`)
	return err
}
