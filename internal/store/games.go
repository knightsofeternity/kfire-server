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
	Hidden          bool
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
// Per player, the total is the imported platform baseline (Steam) plus local
// sessions recorded since the last sync, or all local sessions when there is no
// baseline (see playtime.go). This avoids double-counting Steam time the client
// also observed while still surfacing recent and non-platform play.
func (s *Store) GameLeaderboard(ctx context.Context, gameID string, limit int, visibleOnly bool) ([]LeaderboardEntry, int64, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	boardVis := ""
	totalsVisJoin := ""
	if visibleOnly {
		boardVis = " AND u.sessions_visible"
		totalsVisJoin = "JOIN users vu ON vu.id = u.user_id AND vu.banned_at IS NULL AND vu.sessions_visible"
	}
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		WITH sess AS (
			SELECT user_id, COALESCE(sum(duration_seconds),0)::bigint AS secs, count(*) AS cnt
			FROM game_sessions WHERE game_id = $1 GROUP BY user_id
		),
		ext AS (
			SELECT user_id, sum(total_seconds)::bigint AS base, max(last_synced_at) AS synced
			FROM external_playtime WHERE game_id = $1 GROUP BY user_id
		),
		sess_since AS (
			SELECT s.user_id, COALESCE(sum(s.duration_seconds),0)::bigint AS secs
			FROM game_sessions s JOIN ext ON ext.user_id = s.user_id
			WHERE s.game_id = $1 AND s.started_at > ext.synced
			GROUP BY s.user_id
		),
		players AS (SELECT user_id FROM sess UNION SELECT user_id FROM ext),
		board AS (
			SELECT u.id, u.username, u.avatar_url,
			       (CASE WHEN ext.user_id IS NOT NULL
			             THEN ext.base + COALESCE(sess_since.secs,0)
			             ELSE COALESCE(sess.secs,0) END)::bigint AS total,
			       COALESCE(sess.cnt,0) AS cnt
			FROM players p
			JOIN users u ON u.id = p.user_id AND u.banned_at IS NULL%s
			LEFT JOIN sess ON sess.user_id = p.user_id
			LEFT JOIN ext  ON ext.user_id  = p.user_id
			LEFT JOIN sess_since ON sess_since.user_id = p.user_id
		)
		SELECT id, username, avatar_url, total, cnt
		FROM board WHERE total > 0
		ORDER BY total DESC, cnt DESC
		LIMIT $2`, boardVis), gameID, limit)
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
	err = s.pool.QueryRow(ctx, fmt.Sprintf(`
		WITH sess AS (
			SELECT user_id, COALESCE(sum(duration_seconds),0)::bigint AS secs
			FROM game_sessions WHERE game_id = $1 GROUP BY user_id
		),
		ext AS (
			SELECT user_id, sum(total_seconds)::bigint AS base, max(last_synced_at) AS synced
			FROM external_playtime WHERE game_id = $1 GROUP BY user_id
		),
		sess_since AS (
			SELECT s.user_id, COALESCE(sum(s.duration_seconds),0)::bigint AS secs
			FROM game_sessions s JOIN ext ON ext.user_id = s.user_id
			WHERE s.game_id = $1 AND s.started_at > ext.synced
			GROUP BY s.user_id
		),
		p AS (
			SELECT CASE WHEN ext.user_id IS NOT NULL
			            THEN ext.base + COALESCE(ss.secs,0)
			            ELSE COALESCE(sess.secs,0) END AS secs
			FROM (SELECT user_id FROM sess UNION SELECT user_id FROM ext) u
			%s
			LEFT JOIN sess ON sess.user_id = u.user_id
			LEFT JOIN ext  ON ext.user_id  = u.user_id
			LEFT JOIN sess_since ss ON ss.user_id = u.user_id
		)
		SELECT COALESCE(sum(secs),0)::bigint, count(*) FROM p WHERE secs > 0`, totalsVisJoin),
		gameID).Scan(&totalSeconds, &players)
	return out, totalSeconds, players, err
}

// GameSummary is a played game with org-wide aggregates for the games list.
type GameSummary struct {
	Game         Game
	PlayerCount  int
	TotalSeconds int64
}

// ListPlayedGames returns every game with real activity in the org (a local
// session or imported playtime), alphabetical, with the number of players and
// the cumulative time. Per player the time is the platform baseline plus local
// sessions since the last sync (same merge as the leaderboard, see playtime.go),
// and players with zero time (e.g. owned-but-unplayed Steam games) are excluded.
func (s *Store) ListPlayedGames(ctx context.Context) ([]GameSummary, error) {
	rows, err := s.pool.Query(ctx, `
		WITH sess AS (
			SELECT game_id, user_id, COALESCE(sum(duration_seconds),0)::bigint AS secs
			FROM game_sessions GROUP BY game_id, user_id
		),
		ext AS (
			SELECT game_id, user_id, sum(total_seconds)::bigint AS base, max(last_synced_at) AS synced
			FROM external_playtime GROUP BY game_id, user_id
		),
		sess_since AS (
			SELECT s.game_id, s.user_id, COALESCE(sum(s.duration_seconds),0)::bigint AS secs
			FROM game_sessions s JOIN ext ON ext.game_id = s.game_id AND ext.user_id = s.user_id
			WHERE s.started_at > ext.synced
			GROUP BY s.game_id, s.user_id
		),
		per_player AS (
			SELECT u.game_id,
			       CASE WHEN ext.user_id IS NOT NULL
			            THEN ext.base + COALESCE(ss.secs,0)
			            ELSE COALESCE(sess.secs,0) END AS secs
			FROM (SELECT game_id, user_id FROM sess UNION SELECT game_id, user_id FROM ext) u
			JOIN users usr ON usr.id = u.user_id AND usr.banned_at IS NULL
			LEFT JOIN sess ON sess.game_id = u.game_id AND sess.user_id = u.user_id
			LEFT JOIN ext  ON ext.game_id  = u.game_id  AND ext.user_id  = u.user_id
			LEFT JOIN sess_since ss ON ss.game_id = u.game_id AND ss.user_id = u.user_id
		),
		agg AS (
			SELECT game_id, count(*) AS players, sum(secs)::bigint AS total
			FROM per_player WHERE secs > 0 GROUP BY game_id
		)
		SELECT g.id, g.name, g.slug, g.icon_url, g.cover_url, a.players, a.total
		FROM agg a JOIN games g ON g.id = a.game_id
		WHERE NOT g.hidden
		ORDER BY g.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GameSummary
	for rows.Next() {
		var gs GameSummary
		if err := rows.Scan(&gs.Game.ID, &gs.Game.Name, &gs.Game.Slug,
			&gs.Game.IconURL, &gs.Game.CoverURL, &gs.PlayerCount, &gs.TotalSeconds); err != nil {
			return nil, err
		}
		out = append(out, gs)
	}
	return out, rows.Err()
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
		SELECT id, name, slug, executable_names, platform, icon_url, cover_url, hidden
		FROM games WHERE slug = $1`, slug).
		Scan(&g.ID, &g.Name, &g.Slug, &g.ExecutableNames, &g.Platform, &g.IconURL, &g.CoverURL, &g.Hidden)
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

// SetGameHidden toggles a game's hidden flag. Hidden games are excluded from
// playtime stats and the community games list. Returns ErrNotFound if no row
// matches the id.
func (s *Store) SetGameHidden(ctx context.Context, gameID string, hidden bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE games SET hidden = $2 WHERE id = $1`, gameID, hidden)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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

// GameRecentPlayers returns the members who played this game over the last
// `days` days, by tracked playtime (completed sessions only). Same privacy rules
// as WeeklyLeaderboards: banned members, members who hid their sessions
// (sessions_visible = false), and hidden games are excluded. The undated Steam
// baseline (external_playtime) is intentionally not counted here.
func (s *Store) GameRecentPlayers(ctx context.Context, gameID string, days, limit int) ([]LeaderPlayer, error) {
	if days <= 0 {
		days = 7
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.username, u.avatar_url, sum(gs.duration_seconds)::bigint AS secs
		FROM game_sessions gs
		JOIN users u ON u.id = gs.user_id
		JOIN games g ON g.id = gs.game_id
		WHERE gs.game_id = $1
		  AND gs.ended_at IS NOT NULL
		  AND gs.started_at >= now() - make_interval(days => $2)
		  AND u.banned_at IS NULL
		  AND u.sessions_visible
		  AND NOT g.hidden
		GROUP BY u.id, u.username, u.avatar_url
		HAVING sum(gs.duration_seconds) > 0
		ORDER BY secs DESC, u.username ASC
		LIMIT $3`, gameID, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LeaderPlayer
	for rows.Next() {
		var p LeaderPlayer
		if err := rows.Scan(&p.UserID, &p.Username, &p.AvatarURL, &p.TotalSeconds); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
