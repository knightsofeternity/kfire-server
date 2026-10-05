package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// EpicAccount is the state of a member's Epic link.
type EpicAccount struct {
	Status           string // "ok" | "needs_relink"
	RefreshExpiresAt *time.Time
	LastSyncedAt     *time.Time
}

// EpicTitle is one Epic catalog item, as the library and catalog describe it.
type EpicTitle struct {
	Namespace     string
	CatalogItemID string
	AppName       string
	Title         string
	IsGame        bool
	ImageURL      string
	GameID        *string
}

// SaveEpicLink links (or re-links) a member's Epic account with its sealed
// refresh token, and resets the link to "ok".
func (s *Store) SaveEpicLink(ctx context.Context, userID, accountID, displayName string, refreshEnc []byte, refreshExp time.Time) error {
	if err := s.UpsertLinkedAccount(ctx, userID, LinkedAccount{
		Provider: "epic", ProviderUserID: accountID, DisplayName: &displayName, RefreshTokenEnc: refreshEnc,
	}); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO epic_accounts (user_id, refresh_expires_at, status) VALUES ($1, $2, 'ok')
		ON CONFLICT (user_id) DO UPDATE SET refresh_expires_at = EXCLUDED.refresh_expires_at,
		    status = 'ok', updated_at = now()`, userID, refreshExp)
	return err
}

// EpicRefreshToken returns the member's sealed refresh token.
func (s *Store) EpicRefreshToken(ctx context.Context, userID string) ([]byte, error) {
	var enc []byte
	err := s.pool.QueryRow(ctx, `
		SELECT refresh_token_enc FROM linked_accounts WHERE user_id = $1 AND provider = 'epic'`, userID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return enc, err
}

// UpdateEpicToken stores the refresh token Epic rotated in.
func (s *Store) UpdateEpicToken(ctx context.Context, userID string, refreshEnc []byte, refreshExp time.Time) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE linked_accounts SET refresh_token_enc = $2, updated_at = now()
		WHERE user_id = $1 AND provider = 'epic'`, userID, refreshEnc); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE epic_accounts SET refresh_expires_at = $2, updated_at = now() WHERE user_id = $1`, userID, refreshExp)
	return err
}

// MarkEpicNeedsRelink stops syncing a member whose token Epic refused.
func (s *Store) MarkEpicNeedsRelink(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE epic_accounts SET status = 'needs_relink', updated_at = now() WHERE user_id = $1`, userID)
	return err
}

// MarkEpicSynced records a successful sync.
func (s *Store) MarkEpicSynced(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE epic_accounts SET last_synced_at = now(), updated_at = now() WHERE user_id = $1`, userID)
	return err
}

// EpicAccountFor returns a member's link state, ErrNotFound when unlinked.
func (s *Store) EpicAccountFor(ctx context.Context, userID string) (EpicAccount, error) {
	var a EpicAccount
	err := s.pool.QueryRow(ctx, `
		SELECT status, refresh_expires_at, last_synced_at FROM epic_accounts WHERE user_id = $1`, userID).
		Scan(&a.Status, &a.RefreshExpiresAt, &a.LastSyncedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// EpicLinkedOK lists the members whose link is alive, for the poller.
func (s *Store) EpicLinkedOK(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.user_id FROM epic_accounts e
		JOIN linked_accounts l ON l.user_id = e.user_id AND l.provider = 'epic'
		WHERE e.status = 'ok'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// DeleteEpicLink unlinks Epic: the token and the link state go, imported
// hours stay (like Steam and PSN). ErrNotFound when nothing was linked.
func (s *Store) DeleteEpicLink(ctx context.Context, userID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM epic_accounts WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return s.DeleteLinkedAccount(ctx, userID, "epic")
}

// EpicTitleByKey returns a cached catalog item.
func (s *Store) EpicTitleByKey(ctx context.Context, namespace, catalogItemID string) (EpicTitle, bool, error) {
	t := EpicTitle{Namespace: namespace, CatalogItemID: catalogItemID}
	var img *string
	err := s.pool.QueryRow(ctx, `
		SELECT app_name, title, is_game, image_url, game_id FROM epic_titles
		WHERE namespace = $1 AND catalog_item_id = $2`, namespace, catalogItemID).
		Scan(&t.AppName, &t.Title, &t.IsGame, &img, &t.GameID)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, false, nil
	}
	if img != nil {
		t.ImageURL = *img
	}
	return t, err == nil, err
}

// SaveEpicTitle caches a catalog item.
func (s *Store) SaveEpicTitle(ctx context.Context, t EpicTitle) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO epic_titles (namespace, catalog_item_id, app_name, title, is_game, image_url, game_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		ON CONFLICT (namespace, catalog_item_id) DO UPDATE SET
		    app_name = EXCLUDED.app_name, title = EXCLUDED.title, is_game = EXCLUDED.is_game,
		    image_url = EXCLUDED.image_url, game_id = EXCLUDED.game_id, updated_at = now()`,
		t.Namespace, t.CatalogItemID, t.AppName, t.Title, t.IsGame, t.ImageURL, t.GameID)
	return err
}

// UpsertEpicGame resolves an Epic game to a catalog game by the slug of its
// normalized name (so Epic hours join the PC game of the same name), else a
// new PC entry with the Epic cover. A game without an icon takes the cover.
func (s *Store) UpsertEpicGame(ctx context.Context, normalizedName, imageURL string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx, `
		UPDATE games SET icon_url = COALESCE(icon_url, NULLIF($2, ''))
		WHERE id = (SELECT id FROM games WHERE slug = $1 LIMIT 1)
		RETURNING id, name, slug, icon_url`, steamSlug(normalizedName), imageURL).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			INSERT INTO games (name, slug, executable_names, platform, icon_url)
			VALUES ($1, $2, '{}', 'pc', NULLIF($3, ''))
			ON CONFLICT (slug) DO UPDATE SET icon_url = COALESCE(games.icon_url, EXCLUDED.icon_url)
			RETURNING id, name, slug, icon_url`, normalizedName, steamSlug(normalizedName), imageURL).
			Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	}
	return g, err
}

// GameNamed reports whether a catalog game has this (normalized) name's slug.
func (s *Store) GameNamed(ctx context.Context, normalizedName string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE slug = $1)`, steamSlug(normalizedName)).Scan(&ok)
	return ok, err
}
