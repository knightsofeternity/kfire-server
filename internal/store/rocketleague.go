package store

import (
	"context"
	"time"
)

// RocketLeagueMatch est un match rapporté. Tous les champs sont des faits sur le
// membre lui-même : la feuille de match complète, qui porte le nom des autres
// joueurs, ne quitte jamais sa machine.
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

// RocketLeagueMemberStats est le bilan d'un membre pour un jeu, déjà agrégé par
// la base pour que la page ne télécharge pas chaque match.
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
	PlayTime     int // secondes
	LastPlayedAt time.Time
}

// InsertRocketLeagueMatch écrit un match. Un match déjà envoyé est ignoré en
// silence : la file locale du client peut réémettre après une reconnexion, et la
// contrainte d'unicité sur (user_id, played_at) est ce qui rend cela sûr.
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

// RocketLeagueStatsByGame rend une ligne par membre ayant rapporté un match,
// ordonnée par nombre de matchs puis par nom. Cet ordre n'est PAS un classement :
// la page trie sur ce qu'elle choisit d'afficher.
//
// Les membres bannis sont exclus, comme pour les agrégats Riot, WoW et Hearthstone.
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
		JOIN users u ON u.id = m.user_id
		WHERE m.game_id = $1 AND u.banned_at IS NULL
		GROUP BY m.user_id, u.username, u.avatar_url
		ORDER BY matches DESC, u.username ASC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RocketLeagueMemberStats
	for rows.Next() {
		var s RocketLeagueMemberStats
		if err := rows.Scan(&s.UserID, &s.Username, &s.AvatarURL,
			&s.Matches, &s.Wins, &s.MVPs,
			&s.Goals, &s.Assists, &s.Saves, &s.Shots, &s.Demos, &s.Score,
			&s.PlayTime, &s.LastPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RocketLeagueRecentMatches rend les derniers matchs d'un membre pour un jeu, du
// plus récent au plus ancien.
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
