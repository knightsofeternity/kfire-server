package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GogAccount is the state of a member's GOG link.
type GogAccount struct {
	GalaxyUserID string
	Status       string // "ok" | "needs_relink"
	LastSyncedAt *time.Time
}

// GogTitle is one GOG product, as the public catalog describes it.
type GogTitle struct {
	ProductID string
	Title     string
	IsGame    bool
	ImageURL  string
	GameID    *string
}

// SaveGogLink links (or re-links) a member's GOG account with its sealed
// refresh token, and resets the link to "ok".
func (s *Store) SaveGogLink(ctx context.Context, userID, galaxyUserID, username string, refreshEnc []byte) error {
	if err := s.UpsertLinkedAccount(ctx, userID, LinkedAccount{
		Provider: "gog", ProviderUserID: galaxyUserID, DisplayName: &username, RefreshTokenEnc: refreshEnc,
	}); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO gog_accounts (user_id, galaxy_user_id, status) VALUES ($1, $2, 'ok')
		ON CONFLICT (user_id) DO UPDATE SET galaxy_user_id = EXCLUDED.galaxy_user_id,
		    status = 'ok', updated_at = now()`, userID, galaxyUserID)
	return err
}

// GogRefreshToken returns the member's sealed refresh token.
func (s *Store) GogRefreshToken(ctx context.Context, userID string) ([]byte, error) {
	var enc []byte
	err := s.pool.QueryRow(ctx, `
		SELECT refresh_token_enc FROM linked_accounts WHERE user_id = $1 AND provider = 'gog'`, userID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return enc, err
}

// UpdateGogToken stores the refresh token GOG handed back.
func (s *Store) UpdateGogToken(ctx context.Context, userID string, refreshEnc []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE linked_accounts SET refresh_token_enc = $2, updated_at = now()
		WHERE user_id = $1 AND provider = 'gog'`, userID, refreshEnc)
	return err
}

// MarkGogNeedsRelink stops syncing a member whose token GOG refused.
func (s *Store) MarkGogNeedsRelink(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE gog_accounts SET status = 'needs_relink', updated_at = now() WHERE user_id = $1`, userID)
	return err
}

// MarkGogSynced records a successful sync.
func (s *Store) MarkGogSynced(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE gog_accounts SET last_synced_at = now(), updated_at = now() WHERE user_id = $1`, userID)
	return err
}

// GogAccountFor returns a member's link state, ErrNotFound when unlinked.
func (s *Store) GogAccountFor(ctx context.Context, userID string) (GogAccount, error) {
	var a GogAccount
	err := s.pool.QueryRow(ctx, `
		SELECT galaxy_user_id, status, last_synced_at FROM gog_accounts WHERE user_id = $1`, userID).
		Scan(&a.GalaxyUserID, &a.Status, &a.LastSyncedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// GogLinkedOK lists the members whose link is alive, for the poller.
func (s *Store) GogLinkedOK(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.user_id FROM gog_accounts g
		JOIN linked_accounts l ON l.user_id = g.user_id AND l.provider = 'gog'
		WHERE g.status = 'ok'`)
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

// DeleteGogLink unlinks GOG: the token and the link state go, imported hours
// stay (like Steam and Epic). ErrNotFound when nothing was linked.
func (s *Store) DeleteGogLink(ctx context.Context, userID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM gog_accounts WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return s.DeleteLinkedAccount(ctx, userID, "gog")
}

// GogTitleByID returns a cached product.
func (s *Store) GogTitleByID(ctx context.Context, productID string) (GogTitle, bool, error) {
	t := GogTitle{ProductID: productID}
	var img *string
	err := s.pool.QueryRow(ctx, `
		SELECT title, is_game, image_url, game_id FROM gog_titles WHERE product_id = $1`, productID).
		Scan(&t.Title, &t.IsGame, &img, &t.GameID)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, false, nil
	}
	if img != nil {
		t.ImageURL = *img
	}
	return t, err == nil, err
}

// SaveGogTitle caches a product.
func (s *Store) SaveGogTitle(ctx context.Context, t GogTitle) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO gog_titles (product_id, title, is_game, image_url, game_id)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		ON CONFLICT (product_id) DO UPDATE SET
		    title = EXCLUDED.title, is_game = EXCLUDED.is_game,
		    image_url = EXCLUDED.image_url, game_id = EXCLUDED.game_id, updated_at = now()`,
		t.ProductID, t.Title, t.IsGame, t.ImageURL, t.GameID)
	return err
}

// UpsertGogGame resolves a GOG game to a catalog game, with the same rule as
// Epic: the slug of its normalized name, else a new PC entry with the GOG
// cover.
func (s *Store) UpsertGogGame(ctx context.Context, normalizedName, imageURL string) (Game, error) {
	return s.UpsertEpicGame(ctx, normalizedName, imageURL)
}

// GogGameClient is a game's cached Galaxy client: ID empty when the game has
// none, CheckedAt nil when it was never looked up.
type GogGameClient struct {
	ID, Secret string
	CheckedAt  *time.Time
}

// GogGameClientFor returns a product's cached Galaxy client.
func (s *Store) GogGameClientFor(ctx context.Context, productID string) (GogGameClient, error) {
	var c GogGameClient
	var id, secret *string
	err := s.pool.QueryRow(ctx, `
		SELECT galaxy_client_id, galaxy_client_secret, client_checked_at FROM gog_titles WHERE product_id = $1`, productID).
		Scan(&id, &secret, &c.CheckedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	if id != nil && secret != nil {
		c.ID, c.Secret = *id, *secret
	}
	return c, err
}

// SaveGogGameClient caches a product's Galaxy client, empty for none.
func (s *Store) SaveGogGameClient(ctx context.Context, productID, id, secret string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE gog_titles SET galaxy_client_id = NULLIF($2, ''), galaxy_client_secret = NULLIF($3, ''),
		    client_checked_at = now(), updated_at = now()
		WHERE product_id = $1`, productID, id, secret)
	return err
}
