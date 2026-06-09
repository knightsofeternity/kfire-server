package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Game mirrors a row of the games table.
type Game struct {
	ID              string
	Name            string
	Slug            string
	ExecutableNames []string
	Platform        string
	IconURL         *string
	CoverURL        *string
}

// GameSeed is a normalized entry from an external catalog (Discord).
type GameSeed struct {
	DiscordAppID    string
	Name            string
	Slug            string
	ExecutableNames []string
	IconURL         string
	CoverURL        string
	SteamAppID      string
}

// LeaderboardEntry is one player's standing for a game.
type LeaderboardEntry struct {
	UserID       string
	Username     string
	AvatarURL    *string
	TotalSeconds int64
	SessionCount int
}

// GameLeaderboard returns the top players by playtime for a game, plus the
// org-wide total seconds and the number of distinct players.
//
// Per player, the total is the GREATER of our locally observed sessions and the
// imported platform playtime (Steam playtime_forever) — never their sum, since
// time played through Steam while the KFIRE client is running is counted by
// both. Steam wins for games played through it; our sessions cover games played
// outside any linked platform.
func (s *Store) GameLeaderboard(ctx context.Context, gameID string, limit int) ([]LeaderboardEntry, int64, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx, `
		WITH sess AS (
			SELECT user_id, COALESCE(sum(duration_seconds),0)::bigint AS secs, count(*) AS cnt
			FROM game_sessions WHERE game_id = $1 GROUP BY user_id
		),
		ext AS (
			SELECT user_id, sum(total_seconds)::bigint AS secs
			FROM external_playtime WHERE game_id = $1 GROUP BY user_id
		),
		players AS (SELECT user_id FROM sess UNION SELECT user_id FROM ext)
		SELECT u.id, u.username, u.avatar_url,
		       GREATEST(COALESCE(sess.secs,0), COALESCE(ext.secs,0))::bigint AS total,
		       COALESCE(sess.cnt,0) AS cnt
		FROM players p
		JOIN users u ON u.id = p.user_id AND u.banned_at IS NULL
		LEFT JOIN sess ON sess.user_id = p.user_id
		LEFT JOIN ext  ON ext.user_id  = p.user_id
		ORDER BY total DESC, cnt DESC
		LIMIT $2`, gameID, limit)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var out []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.AvatarURL, &e.TotalSeconds, &e.SessionCount); err != nil {
			return nil, 0, 0, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}

	var totalSeconds int64
	var players int
	err = s.pool.QueryRow(ctx, `
		WITH sess AS (
			SELECT user_id, COALESCE(sum(duration_seconds),0)::bigint AS secs
			FROM game_sessions WHERE game_id = $1 GROUP BY user_id
		),
		ext AS (
			SELECT user_id, sum(total_seconds)::bigint AS secs
			FROM external_playtime WHERE game_id = $1 GROUP BY user_id
		),
		p AS (
			SELECT GREATEST(COALESCE(sess.secs,0), COALESCE(ext.secs,0)) AS secs
			FROM (SELECT user_id FROM sess UNION SELECT user_id FROM ext) u
			LEFT JOIN sess USING (user_id)
			LEFT JOIN ext  USING (user_id)
		)
		SELECT COALESCE(sum(secs),0)::bigint, count(*) FROM p`,
		gameID).Scan(&totalSeconds, &players)
	return out, totalSeconds, players, err
}

// CountGames returns the catalog size.
func (s *Store) CountGames(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM games`).Scan(&n)
	return n, err
}

// ListGames returns the whole catalog (the desktop client downloads it to
// match local processes).
func (s *Store) ListGames(ctx context.Context) ([]Game, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, executable_names, platform, icon_url
		FROM games ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug, &g.ExecutableNames, &g.Platform, &g.IconURL); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// GetGameBySlug fetches one game.
func (s *Store) GetGameBySlug(ctx context.Context, slug string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, executable_names, platform, icon_url, cover_url
		FROM games WHERE slug = $1`, slug).
		Scan(&g.ID, &g.Name, &g.Slug, &g.ExecutableNames, &g.Platform, &g.IconURL, &g.CoverURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Game{}, ErrNotFound
	}
	return g, err
}

// GetGameByID fetches one game by id (used by the image proxy).
func (s *Store) GetGameByID(ctx context.Context, id string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, executable_names, platform, icon_url, cover_url
		FROM games WHERE id = $1`, id).
		Scan(&g.ID, &g.Name, &g.Slug, &g.ExecutableNames, &g.Platform, &g.IconURL, &g.CoverURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Game{}, ErrNotFound
	}
	return g, err
}

// UpsertGames inserts or updates seeded games keyed on discord_app_id.
// Slugs are stable: an existing game keeps its slug across re-syncs; new
// games get a unique slug (name collisions are suffixed with the app id).
func (s *Store) UpsertGames(ctx context.Context, seeds []GameSeed) (int, error) {
	// Existing slug assignments, to keep slugs stable and unique.
	rows, err := s.pool.Query(ctx, `SELECT slug, COALESCE(discord_app_id, '') FROM games`)
	if err != nil {
		return 0, err
	}
	slugByApp := make(map[string]string)
	taken := make(map[string]struct{})
	for rows.Next() {
		var slug, appID string
		if err := rows.Scan(&slug, &appID); err != nil {
			rows.Close()
			return 0, err
		}
		taken[slug] = struct{}{}
		if appID != "" {
			slugByApp[appID] = slug
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	const chunkSize = 500
	upserted := 0
	for start := 0; start < len(seeds); start += chunkSize {
		chunk := seeds[start:min(start+chunkSize, len(seeds))]

		batch := &pgx.Batch{}
		for _, seed := range chunk {
			slug, exists := slugByApp[seed.DiscordAppID]
			if !exists {
				slug = seed.Slug
				if _, dup := taken[slug]; dup {
					slug = fmt.Sprintf("%s-%s", slug, seed.DiscordAppID)
				}
				taken[slug] = struct{}{}
				slugByApp[seed.DiscordAppID] = slug
			}

			batch.Queue(`
				INSERT INTO games (name, slug, executable_names, platform, icon_url, cover_url, discord_app_id, steam_app_id)
				VALUES ($1, $2, $3, 'pc', NULLIF($4, ''), NULLIF($5, ''), $6, NULLIF($7, ''))
				ON CONFLICT (discord_app_id) DO UPDATE SET
					name             = EXCLUDED.name,
					executable_names = EXCLUDED.executable_names,
					icon_url         = EXCLUDED.icon_url,
					cover_url        = EXCLUDED.cover_url,
					steam_app_id     = EXCLUDED.steam_app_id`,
				seed.Name, slug, seed.ExecutableNames, seed.IconURL, seed.CoverURL, seed.DiscordAppID, seed.SteamAppID)
		}

		if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
			return upserted, fmt.Errorf("upsert games batch at %d: %w", start, err)
		}
		upserted += len(chunk)
	}
	return upserted, nil
}
