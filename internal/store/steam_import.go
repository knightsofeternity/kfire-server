package store

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// LinkedUser is a (user, platform id) pair for a given provider.
type LinkedUser struct {
	UserID         string
	ProviderUserID string
}

// ListLinkedByProvider returns every account linked for a provider (the poller
// iterates these to sync).
func (s *Store) ListLinkedByProvider(ctx context.Context, provider string) ([]LinkedUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, provider_user_id FROM linked_accounts WHERE provider = $1`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LinkedUser
	for rows.Next() {
		var u LinkedUser
		if err := rows.Scan(&u.UserID, &u.ProviderUserID); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GamesBySteamAppID resolves Steam AppIDs to catalog games. AppIDs not in the
// catalog are simply absent from the result.
func (s *Store) GamesBySteamAppID(ctx context.Context, appIDs []string) (map[string]Game, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT steam_app_id, id, name, slug, icon_url
		FROM games WHERE steam_app_id = ANY($1)`, appIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]Game)
	for rows.Next() {
		var appID string
		var g Game
		if err := rows.Scan(&appID, &g.ID, &g.Name, &g.Slug, &g.IconURL); err != nil {
			return nil, err
		}
		out[appID] = g
	}
	return out, rows.Err()
}

// UpsertSteamGame resolves a Steam-owned game to a catalog game so its playtime
// and achievements import even when the Discord catalog has a wrong or missing
// Steam AppID. It matches by AppID, then by slug (adopting the entry and
// correcting its AppID), and finally creates a new entry from the Steam data.
// Created entries have no executables, so they are not locally detectable; they
// exist for playtime, achievements and the games list.
func (s *Store) UpsertSteamGame(ctx context.Context, appID, name, iconURL string) (Game, error) {
	var g Game

	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, icon_url FROM games WHERE steam_app_id = $1 LIMIT 1`, appID).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err == nil {
		return g, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return g, err
	}

	slug := steamSlug(name)
	err = s.pool.QueryRow(ctx, `
		UPDATE games
		SET steam_app_id = $1, icon_url = COALESCE(icon_url, NULLIF($3, ''))
		WHERE slug = $2
		RETURNING id, name, slug, icon_url`,
		appID, slug, iconURL).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err == nil {
		return g, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return g, err
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO games (name, slug, executable_names, platform, icon_url, steam_app_id)
		VALUES ($1, $2, '{}', 'pc', NULLIF($3, ''), $4)
		RETURNING id, name, slug, icon_url`,
		name, slug, iconURL, appID).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	return g, err
}

var steamSlugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// steamSlug mirrors games.Slugify without importing that package (which would
// create an import cycle: the games package depends on this store package).
func steamSlug(name string) string {
	s := strings.ToLower(name)
	s = steamSlugInvalid.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "game"
	}
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}

// UpsertExternalPlaytime stores a member's lifetime playtime for one game.
func (s *Store) UpsertExternalPlaytime(ctx context.Context, userID, provider, gameID string, totalSeconds int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO external_playtime (user_id, provider, game_id, total_seconds, last_synced_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id, provider, game_id) DO UPDATE SET
			total_seconds  = EXCLUDED.total_seconds,
			last_synced_at = now()`,
		userID, provider, gameID, totalSeconds)
	return err
}

// AchievementRow is one unlocked achievement to persist.
type AchievementRow struct {
	GameID      string
	APIName     string
	DisplayName string
	IconURL     string
	UnlockedAt  time.Time
}

// UpsertAchievements stores unlocked achievements for a user+provider+game.
func (s *Store) UpsertAchievements(ctx context.Context, userID, provider string, rows []AchievementRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, a := range rows {
		batch.Queue(`
			INSERT INTO achievements
				(user_id, provider, game_id, api_name, display_name, icon_url, unlocked_at)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7)
			ON CONFLICT (user_id, provider, game_id, api_name) DO UPDATE SET
				display_name = EXCLUDED.display_name,
				icon_url     = EXCLUDED.icon_url,
				unlocked_at  = EXCLUDED.unlocked_at`,
			userID, provider, a.GameID, a.APIName, a.DisplayName, a.IconURL, a.UnlockedAt)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// Achievement is an unlocked achievement joined with its game, for display.
type Achievement struct {
	Game        Game
	APIName     string
	DisplayName *string
	IconURL     *string
	UnlockedAt  time.Time
}

// RecentAchievements returns a user's most recently unlocked achievements.
func (s *Store) RecentAchievements(ctx context.Context, userID string, limit int) ([]Achievement, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.api_name, a.display_name, a.icon_url, a.unlocked_at,
		       g.id, g.name, g.slug, g.icon_url
		FROM achievements a
		JOIN games g ON g.id = a.game_id
		WHERE a.user_id = $1
		ORDER BY a.unlocked_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Achievement
	for rows.Next() {
		var a Achievement
		if err := rows.Scan(&a.APIName, &a.DisplayName, &a.IconURL, &a.UnlockedAt,
			&a.Game.ID, &a.Game.Name, &a.Game.Slug, &a.Game.IconURL); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AchievementCount returns the total unlocked achievements of a user.
func (s *Store) AchievementCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM achievements WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}
