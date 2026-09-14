package store

import (
	"context"
	"time"
)

// HearthstoneMatch is one reported match. Placement and Turns are pointers
// because a match is recorded even when a secondary field was unreadable in the
// game's log.
type HearthstoneMatch struct {
	UserID    string
	GameID    string
	Mode      string
	Result    string
	Turns     *int
	Placement *int
	PlayedAt  time.Time
}

// HearthstoneMemberStats is one member's record for a game, already aggregated
// by the database so the page does not download every match.
type HearthstoneMemberStats struct {
	UserID       string
	Username     string
	AvatarURL    *string
	Matches      int
	Wins         int
	Ranked       int // matches carrying a placement
	AvgPlacement *float64
	Top4         int
	LastPlayedAt time.Time
}

// InsertHearthstoneMatch records one match. A match the client already sent is
// silently ignored: the local queue may resend after a reconnection, and the
// unique constraint on (user_id, played_at) is what makes that safe.
func (s *Store) InsertHearthstoneMatch(ctx context.Context, m HearthstoneMatch) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO hearthstone_matches
			(user_id, game_id, mode, result, turns, placement, played_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, played_at) DO NOTHING`,
		m.UserID, m.GameID, m.Mode, m.Result, m.Turns, m.Placement, m.PlayedAt)
	return err
}

// HearthstoneStatsByGame returns one row per member who reported a match,
// ordered by match count then name. That ordering is NOT a ranking: the page
// sorts on what it chooses to display.
//
// Banned members are excluded, matching the Riot and WoW aggregates.
func (s *Store) HearthstoneStatsByGame(ctx context.Context, gameID string) ([]HearthstoneMemberStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.user_id, u.username, u.avatar_url,
		       count(*)                                 AS matches,
		       count(*) FILTER (WHERE m.result = 'win') AS wins,
		       count(m.placement)                       AS ranked,
		       avg(m.placement)                         AS avg_placement,
		       count(*) FILTER (WHERE m.placement <= 4) AS top4,
		       max(m.played_at)                         AS last_played_at
		FROM hearthstone_matches m
		JOIN users u ON u.id = m.user_id AND u.banned_at IS NULL
		WHERE m.game_id = $1
		GROUP BY m.user_id, u.username, u.avatar_url
		ORDER BY count(*) DESC, u.username ASC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HearthstoneMemberStats
	for rows.Next() {
		var r HearthstoneMemberStats
		if err := rows.Scan(&r.UserID, &r.Username, &r.AvatarURL, &r.Matches,
			&r.Wins, &r.Ranked, &r.AvgPlacement, &r.Top4, &r.LastPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
