package store

import "context"

// LeaderPlayer is a member's total tracked playtime over a window.
type LeaderPlayer struct {
	UserID       string
	Username     string
	AvatarURL    *string
	TotalSeconds int64
}

// LeaderGame is a game's tracked playtime and distinct players over a window.
type LeaderGame struct {
	Game         Game
	TotalSeconds int64
	PlayerCount  int
}

// WeeklyLeaderboardsResult bundles both tops for one window.
type WeeklyLeaderboardsResult struct {
	WindowDays int
	TopPlayers []LeaderPlayer
	TopGames   []LeaderGame
}

// WeeklyLeaderboards computes the top players (by tracked playtime) and the top
// games (by tracked playtime and distinct players) over the last `days` days.
// Privacy is honored: banned members, members who hid their sessions
// (sessions_visible = false), and hidden games are excluded, and a hidden
// member's time never inflates a game total. Only completed sessions count.
func (s *Store) WeeklyLeaderboards(ctx context.Context, days, limit int) (WeeklyLeaderboardsResult, error) {
	res := WeeklyLeaderboardsResult{WindowDays: days}

	prows, err := s.pool.Query(ctx, `
		SELECT u.id, u.username, u.avatar_url, sum(gs.duration_seconds)::bigint AS secs
		FROM game_sessions gs
		JOIN users u ON u.id = gs.user_id
		JOIN games g ON g.id = gs.game_id
		WHERE gs.ended_at IS NOT NULL
		  AND gs.started_at >= now() - make_interval(days => $1)
		  AND u.banned_at IS NULL
		  AND u.sessions_visible
		  AND NOT g.hidden
		GROUP BY u.id, u.username, u.avatar_url
		HAVING sum(gs.duration_seconds) > 0
		ORDER BY secs DESC, u.username ASC
		LIMIT $2`, days, limit)
	if err != nil {
		return res, err
	}
	for prows.Next() {
		var p LeaderPlayer
		if err := prows.Scan(&p.UserID, &p.Username, &p.AvatarURL, &p.TotalSeconds); err != nil {
			prows.Close()
			return res, err
		}
		res.TopPlayers = append(res.TopPlayers, p)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return res, err
	}

	grows, err := s.pool.Query(ctx, `
		SELECT g.id, g.slug, g.name, g.icon_url, g.cover_url,
		       sum(gs.duration_seconds)::bigint AS secs,
		       count(DISTINCT gs.user_id)::int AS players
		FROM game_sessions gs
		JOIN games g ON g.id = gs.game_id
		JOIN users u ON u.id = gs.user_id
		WHERE gs.ended_at IS NOT NULL
		  AND gs.started_at >= now() - make_interval(days => $1)
		  AND NOT g.hidden
		  AND u.banned_at IS NULL
		  AND u.sessions_visible
		GROUP BY g.id, g.slug, g.name, g.icon_url, g.cover_url
		HAVING sum(gs.duration_seconds) > 0
		ORDER BY secs DESC, players DESC, g.name ASC
		LIMIT $2`, days, limit)
	if err != nil {
		return res, err
	}
	for grows.Next() {
		var lg LeaderGame
		if err := grows.Scan(&lg.Game.ID, &lg.Game.Slug, &lg.Game.Name,
			&lg.Game.IconURL, &lg.Game.CoverURL, &lg.TotalSeconds, &lg.PlayerCount); err != nil {
			grows.Close()
			return res, err
		}
		res.TopGames = append(res.TopGames, lg)
	}
	grows.Close()
	if err := grows.Err(); err != nil {
		return res, err
	}
	return res, nil
}
