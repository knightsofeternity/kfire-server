package store

import (
	"context"
	"time"
)

// RocketLeagueMatch is a reported match. Every field is a fact about the
// member themself: the full match sheet, which names the other players,
// never leaves their machine.
type RocketLeagueMatch struct {
	UserID          string
	GameID          string
	Playlist        int
	TeamSize        int
	PlayerTeam      int
	TeamBlueScore   int
	TeamOrangeScore int
	Result          string
	Goals           int
	Assists         int
	Saves           int
	Shots           int
	Score           int
	Demos           int
	MVP             bool
	DurationSeconds int
	PlayedAt        time.Time
}

// RocketLeagueMemberStats is a member's record for a game, already
// aggregated by the database so the page does not download every match.
type RocketLeagueMemberStats struct {
	UserID       string
	Username     string
	AvatarURL    *string
	Matches      int
	Wins         int
	MVPs         int
	Goals        int
	Assists      int
	Saves        int
	Shots        int
	Demos        int
	Score        int
	PlayTime     int // seconds
	LastPlayedAt time.Time
}

// InsertRocketLeagueMatch writes a match. An already-sent match is silently
// ignored: the client's local queue can re-emit after a reconnect, and the
// uniqueness constraint on (user_id, played_at) is what makes that safe.
func (s *Store) InsertRocketLeagueMatch(ctx context.Context, m RocketLeagueMatch) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO rocket_league_matches
			(user_id, game_id, playlist, team_size, player_team,
			 team_blue_score, team_orange_score, result,
			 goals, assists, saves, shots, score, demos,
			 mvp, duration_seconds, played_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (user_id, played_at) DO NOTHING`,
		m.UserID, m.GameID, m.Playlist, m.TeamSize, m.PlayerTeam,
		m.TeamBlueScore, m.TeamOrangeScore, m.Result,
		m.Goals, m.Assists, m.Saves, m.Shots, m.Score, m.Demos,
		m.MVP, m.DurationSeconds, m.PlayedAt)
	return err
}

// RocketLeagueStatsByGame returns one row per member who reported a match,
// ordered by match count then by name. This order is NOT a leaderboard: the
// page sorts on whatever it chooses to display.
//
// Banned members are excluded, as for the Riot, WoW and Hearthstone aggregates.
func (s *Store) RocketLeagueStatsByGame(ctx context.Context, gameID string) ([]RocketLeagueMemberStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.user_id, u.username, u.avatar_url,
		       count(*)                                 AS matches,
		       count(*) FILTER (WHERE m.result = 'win') AS wins,
		       count(*) FILTER (WHERE m.mvp)            AS mvps,
		       coalesce(sum(m.goals), 0)                AS goals,
		       coalesce(sum(m.assists), 0)              AS assists,
		       coalesce(sum(m.saves), 0)                AS saves,
		       coalesce(sum(m.shots), 0)                AS shots,
		       coalesce(sum(m.demos), 0)                AS demos,
		       coalesce(sum(m.score), 0)                AS score,
		       coalesce(sum(m.duration_seconds), 0)     AS play_time,
		       max(m.played_at)                         AS last_played_at
		FROM rocket_league_matches m
		JOIN users u ON u.id = m.user_id AND u.banned_at IS NULL
		WHERE m.game_id = $1
		GROUP BY m.user_id, u.username, u.avatar_url
		ORDER BY matches DESC, u.username ASC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RocketLeagueMemberStats
	for rows.Next() {
		var r RocketLeagueMemberStats
		if err := rows.Scan(&r.UserID, &r.Username, &r.AvatarURL,
			&r.Matches, &r.Wins, &r.MVPs,
			&r.Goals, &r.Assists, &r.Saves, &r.Shots, &r.Demos, &r.Score,
			&r.PlayTime, &r.LastPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RocketLeagueRecentMatches returns a member's latest matches for a game,
// most recent first.
func (s *Store) RocketLeagueRecentMatches(ctx context.Context, userID, gameID string, limit int) ([]RocketLeagueMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT playlist, team_size, player_team, team_blue_score, team_orange_score,
		       result, goals, assists, saves, shots, score, demos, mvp,
		       duration_seconds, played_at
		FROM rocket_league_matches
		WHERE user_id = $1 AND game_id = $2
		ORDER BY played_at DESC
		LIMIT $3`, userID, gameID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RocketLeagueMatch
	for rows.Next() {
		m := RocketLeagueMatch{UserID: userID, GameID: gameID}
		if err := rows.Scan(&m.Playlist, &m.TeamSize, &m.PlayerTeam,
			&m.TeamBlueScore, &m.TeamOrangeScore, &m.Result,
			&m.Goals, &m.Assists, &m.Saves, &m.Shots, &m.Score, &m.Demos,
			&m.MVP, &m.DurationSeconds, &m.PlayedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
