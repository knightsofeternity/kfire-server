package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// RocketLeagueMatch is a reported match. Every field is a fact about the
// member themself: the full match sheet, which names the other players,
// never leaves their machine.
type RocketLeagueMatch struct {
	UserID          string
	GameID          string
	Playlist        *int
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
	// ID is set when the match is read back, empty when it is written.
	ID string
	// MatchKey and Others are nil for a report from a client older than the
	// scoreboard, and for every match recorded before it.
	MatchKey *string
	Others   []RocketLeaguePlayer
}

// RocketLeaguePlayer is one player of a match other than the reporting
// member. Numbers and one flag: there is no field able to hold a name.
type RocketLeaguePlayer struct {
	Team    int
	Score   int
	Goals   int
	Assists int
	Saves   int
	Shots   int
	Demos   int
	Left    bool
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

// duplicateWindow is how close two identical matches must be for the second
// to be treated as a repeat of the first rather than a real game.
//
// Three minutes is far shorter than any real match and far longer than the
// gap between a client's two reports, so it cannot swallow a genuine game: a
// member would have to finish a second match with the very same score AND the
// very same six statistics within three minutes of the first.
const duplicateWindow = 3 * time.Minute

// InsertRocketLeagueMatch writes a match, unless the member has just reported
// exactly the same one.
//
// Two guards, against two different mistakes. The uniqueness on
// (user_id, played_at) covers a client re-sending a queued match after a
// reconnect, which carries the same timestamp.
//
// The second guard covers something the first cannot see. The client stamps
// played_at when it REPORTS, not when the match ended, so a match reported
// twice a few seconds apart carries two different timestamps and slips past
// any uniqueness constraint. That happens today: the game signals the end of a
// match twice, as MatchEnded and again as MatchDestroyed, and clients up to
// v0.6.0-beta.3 report on both -- the second time with a duration reset to
// near zero. This guard is what keeps those out of members' statistics without
// waiting for every client to be updated.
//
// The match and its other players are written in one transaction. A match
// the guards turn away writes nothing at all, players included.
func (s *Store) InsertRocketLeagueMatch(ctx context.Context, m RocketLeagueMatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO rocket_league_matches
			(user_id, game_id, playlist, team_size, player_team,
			 team_blue_score, team_orange_score, result,
			 goals, assists, saves, shots, score, demos,
			 mvp, duration_seconds, played_at, match_key)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $19
		WHERE NOT EXISTS (
			SELECT 1 FROM rocket_league_matches
			 WHERE user_id = $1
			   AND played_at > $17::timestamptz - $18::interval
			   AND result = $8
			   AND team_blue_score = $6
			   AND team_orange_score = $7
			   AND goals = $9
			   AND assists = $10
			   AND saves = $11
			   AND shots = $12
			   AND score = $13
			   AND demos = $14
		)
		ON CONFLICT (user_id, played_at) DO NOTHING
		RETURNING id`,
		m.UserID, m.GameID, m.Playlist, m.TeamSize, m.PlayerTeam,
		m.TeamBlueScore, m.TeamOrangeScore, m.Result,
		m.Goals, m.Assists, m.Saves, m.Shots, m.Score, m.Demos,
		m.MVP, m.DurationSeconds, m.PlayedAt, duplicateWindow, m.MatchKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Turned away by a guard: not an error, the match is already there.
		return nil
	}
	if err != nil {
		return err
	}
	for i, p := range m.Others {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rocket_league_match_players
				(match_id, position, team, score, goals, assists, saves, shots, demos, left_early)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			id, i, p.Team, p.Score, p.Goals, p.Assists, p.Saves, p.Shots, p.Demos, p.Left); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
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
