package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// NintendoBot is the instance's Nintendo Switch bot, as stored. Secrets stay sealed.
type NintendoBot struct {
	SessionEnc       []byte
	Nickname         *string
	NsaID            *string
	Status           string
	LastError        *string
	SessionSetAt     *time.Time
	LastOKAt         *time.Time
	LoginState       *string
	LoginVerifierEnc []byte
	LoginStartedAt   *time.Time
}

// GetNintendoBot returns the bot row, or ErrNotFound before any login started.
func (s *Store) GetNintendoBot(ctx context.Context) (NintendoBot, error) {
	var b NintendoBot
	err := s.pool.QueryRow(ctx, `
		SELECT session_enc, nickname, nsa_id, status, last_error, session_set_at, last_ok_at,
		       login_state, login_verifier_enc, login_started_at
		FROM nintendo_bot WHERE id = 1`).Scan(&b.SessionEnc, &b.Nickname, &b.NsaID, &b.Status,
		&b.LastError, &b.SessionSetAt, &b.LastOKAt, &b.LoginState, &b.LoginVerifierEnc, &b.LoginStartedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// StartNintendoLogin records a pending login (its state and sealed verifier).
func (s *Store) StartNintendoLogin(ctx context.Context, state string, verifierEnc []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nintendo_bot (id, login_state, login_verifier_enc, login_started_at)
		VALUES (1, $1, $2, now())
		ON CONFLICT (id) DO UPDATE SET login_state = $1, login_verifier_enc = $2, login_started_at = now()`,
		state, verifierEnc)
	return err
}

// SetNintendoSession stores a proven session token and the bot's identity,
// and forgets the pending login.
func (s *Store) SetNintendoSession(ctx context.Context, sessionEnc []byte, nickname, nsaID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nintendo_bot (id, session_enc, nickname, nsa_id, status, session_set_at, last_ok_at)
		VALUES (1, $1, $2, $3, 'ok', now(), now())
		ON CONFLICT (id) DO UPDATE SET session_enc = $1, nickname = $2, nsa_id = $3, status = 'ok',
		    last_error = NULL, session_set_at = now(), last_ok_at = now(),
		    login_state = NULL, login_verifier_enc = NULL, login_started_at = NULL`,
		sessionEnc, nickname, nsaID)
	return err
}

// MarkNintendoNeedsLogin records that Nintendo refused the session token.
func (s *Store) MarkNintendoNeedsLogin(ctx context.Context, reason string) error {
	_, err := s.pool.Exec(ctx, `UPDATE nintendo_bot SET status = 'needs_login', last_error = $1 WHERE id = 1`, reason)
	return err
}

// MarkNintendoOK stamps a successful pass.
func (s *Store) MarkNintendoOK(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `UPDATE nintendo_bot SET last_ok_at = now(), last_error = NULL WHERE id = 1 AND status = 'ok'`)
	return err
}

// UpsertNintendoGame resolves a Switch game to a catalog game: by its eShop
// title id, then by the slug of its normalized name (so a Switch edition
// joins the PC game and their hours add up), else a new nintendo entry with
// the eShop cover. A game without an icon takes the cover. titleID may be
// empty when Nintendo gave no shop link: the game is then found by name only.
func (s *Store) UpsertNintendoGame(ctx context.Context, titleID, normalizedName, imageURL string) (Game, error) {
	var g Game
	err := pgx.ErrNoRows
	if titleID != "" {
		err = s.pool.QueryRow(ctx, `
			SELECT g.id, g.name, g.slug, g.icon_url FROM nintendo_titles t JOIN games g ON g.id = t.game_id
			WHERE t.title_id = $1`, titleID).Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			INSERT INTO games (name, slug, executable_names, platform, icon_url)
			VALUES ($1, $2, '{}', 'nintendo', NULLIF($3, ''))
			ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
			RETURNING id, name, slug, icon_url`, normalizedName, steamSlug(normalizedName), imageURL).
			Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	}
	if err != nil {
		return g, err
	}
	if g.IconURL == nil && imageURL != "" {
		if _, err := s.pool.Exec(ctx, `UPDATE games SET icon_url = $2 WHERE id = $1 AND icon_url IS NULL`, g.ID, imageURL); err != nil {
			return g, err
		}
		g.IconURL = &imageURL
	}
	if titleID != "" {
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO nintendo_titles (title_id, game_id) VALUES ($1, $2)
			ON CONFLICT (title_id) DO NOTHING`, titleID, g.ID); err != nil {
			return g, err
		}
	}
	return g, nil
}

// SetNintendoFriendCode records a member's friend code, normalized.
func (s *Store) SetNintendoFriendCode(ctx context.Context, userID, code string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nintendo_accounts (user_id, friend_code) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET friend_code = $2`, userID, code)
	return err
}

// DeleteNintendoFriendCode forgets it on unlink.
func (s *Store) DeleteNintendoFriendCode(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM nintendo_accounts WHERE user_id = $1`, userID)
	return err
}

// NintendoFriendCode returns a member's friend code, or "".
func (s *Store) NintendoFriendCode(ctx context.Context, userID string) (string, error) {
	var code string
	err := s.pool.QueryRow(ctx, `SELECT friend_code FROM nintendo_accounts WHERE user_id = $1`, userID).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return code, err
}

// NintendoLibraryDue reports whether a member's play log was never imported or
// not for `every`.
func (s *Store) NintendoLibraryDue(ctx context.Context, userID string, every time.Duration) (bool, error) {
	var at time.Time
	err := s.pool.QueryRow(ctx, `SELECT synced_at FROM nintendo_library_sync WHERE user_id = $1`, userID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return err == nil && time.Since(at) >= every, err
}

func (s *Store) MarkNintendoLibrarySynced(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nintendo_library_sync (user_id, synced_at) VALUES ($1, now())
		ON CONFLICT (user_id) DO UPDATE SET synced_at = now()`, userID)
	return err
}

// DeleteNintendoBotForTests empties nintendo_bot. Tests only.
func (s *Store) DeleteNintendoBotForTests(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM nintendo_bot`)
	return err
}
