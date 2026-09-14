package store

import (
	"context"
	"time"
)

// HearthstoneMatch is one reported match. Placement, Turns and HeroCardID are
// pointers because a match is recorded even when a secondary field was
// unreadable in the game's log.
type HearthstoneMatch struct {
	UserID     string
	GameID     string
	Mode       string
	Result     string
	Turns      *int
	Placement  *int
	HeroCardID *string
	PlayedAt   time.Time
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
			(user_id, game_id, mode, result, turns, placement, hero_card_id, played_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, played_at) DO NOTHING`,
		m.UserID, m.GameID, m.Mode, m.Result, m.Turns, m.Placement,
		m.HeroCardID, m.PlayedAt)
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

// HearthstoneHeroStats is the guild's record with one Battlegrounds hero.
type HearthstoneHeroStats struct {
	HeroCardID   string
	Matches      int
	Players      int
	AvgPlacement float64
	Top4         int
	Wins         int
}

// heroBaseID folds a hero skin onto the hero it dresses. A member who owns the
// golden Xyrella reports BG20_HERO_101_SKIN_G where another reports
// BG20_HERO_101; those are the same hero and must aggregate together. The skin
// stays in the row, so nothing is lost.
const heroBaseID = `regexp_replace(m.hero_card_id, '_SKIN_[A-Za-z0-9]+$', '')`

// HearthstoneHeroesByGame returns the guild's record per Battlegrounds hero,
// most played first.
//
// Most played, NOT best average: with a handful of matches an average is noise,
// and a hero played once and won would otherwise top the list for good. How
// often the guild picks a hero is a fact; how good it is at it needs volume.
//
// Only matches carrying a placement count: a hero without one says nothing
// about how the game went. Banned members are excluded, as everywhere else.
func (s *Store) HearthstoneHeroesByGame(ctx context.Context, gameID string) ([]HearthstoneHeroStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+heroBaseID+`                            AS hero,
		       count(*)                                  AS matches,
		       count(DISTINCT m.user_id)                 AS players,
		       avg(m.placement)                          AS avg_placement,
		       count(*) FILTER (WHERE m.placement <= 4)  AS top4,
		       count(*) FILTER (WHERE m.result = 'win')  AS wins
		FROM hearthstone_matches m
		JOIN users u ON u.id = m.user_id AND u.banned_at IS NULL
		WHERE m.game_id = $1 AND m.hero_card_id IS NOT NULL AND m.placement IS NOT NULL
		GROUP BY `+heroBaseID+`
		ORDER BY count(*) DESC, avg(m.placement) ASC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HearthstoneHeroStats
	for rows.Next() {
		var r HearthstoneHeroStats
		if err := rows.Scan(&r.HeroCardID, &r.Matches, &r.Players,
			&r.AvgPlacement, &r.Top4, &r.Wins); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
