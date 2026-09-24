package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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
	// Rating and RatingAfter come from Hearthstone Deck Tracker, when the
	// member enabled the option and HDT recorded the game. Usually absent.
	Rating      *int
	RatingAfter *int
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
	// LastBGPlayedAt is when the member's last Battlegrounds match ended, nil
	// when they never played one. HDT only ever rates Battlegrounds, so a
	// rating is only stale against THIS instant, never against a constructed
	// match played since.
	LastBGPlayedAt *time.Time
}

// InsertHearthstoneMatch records one match. A match the client already sent is
// silently ignored: the local queue may resend after a reconnection, and the
// unique constraint on (user_id, played_at) is what makes that safe.
func (s *Store) InsertHearthstoneMatch(ctx context.Context, m HearthstoneMatch) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO hearthstone_matches
			(user_id, game_id, mode, result, turns, placement, hero_card_id, played_at,
			 rating, rating_after)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, played_at) DO NOTHING`,
		m.UserID, m.GameID, m.Mode, m.Result, m.Turns, m.Placement,
		m.HeroCardID, m.PlayedAt, m.Rating, m.RatingAfter)
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
		       max(m.played_at)                         AS last_played_at,
		       max(m.played_at) FILTER (WHERE m.mode = 'battlegrounds') AS last_bg_played_at
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
			&r.Wins, &r.Ranked, &r.AvgPlacement, &r.Top4, &r.LastPlayedAt,
			&r.LastBGPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// HearthstoneHeroRecord is a record with one Battlegrounds hero, whoever it
// belongs to.
type HearthstoneHeroRecord struct {
	HeroCardID   string
	Matches      int
	AvgPlacement float64
	Top4         int
	Wins         int
}

// HearthstoneHeroStats is the guild's record with one hero, so it also says how
// many members played it. A single member's record has no such field.
type HearthstoneHeroStats struct {
	HearthstoneHeroRecord
	Players int
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

// HearthstoneMemberTotals is one member's record for a game, all modes counted.
type HearthstoneMemberTotals struct {
	Matches      int
	Wins         int
	Ranked       int // matches carrying a placement
	AvgPlacement *float64
	Top4         int
}

// HearthstonePlacementCount is how often a member finished in one place.
type HearthstonePlacementCount struct {
	Placement int
	Matches   int
}

// HearthstoneMatchRow is one match as the member's own page shows it.
type HearthstoneMatchRow struct {
	PlayedAt    time.Time
	Mode        string
	Result      string
	Turns       *int
	Placement   *int
	HeroCardID  *string
	Rating      *int
	RatingAfter *int
}

// HearthstoneMemberTotalsFor returns one member's record for a game.
func (s *Store) HearthstoneMemberTotalsFor(ctx context.Context, userID, gameID string) (HearthstoneMemberTotals, error) {
	var t HearthstoneMemberTotals
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE result = 'win'),
		       count(placement),
		       avg(placement),
		       count(*) FILTER (WHERE placement <= 4)
		FROM hearthstone_matches
		WHERE user_id = $1 AND game_id = $2`, userID, gameID).
		Scan(&t.Matches, &t.Wins, &t.Ranked, &t.AvgPlacement, &t.Top4)
	return t, err
}

// HearthstonePlacementsFor returns how often a member finished in each place.
//
// Only places actually reached come back; the page fills the gaps, so a member
// who never finished eighth still gets an eighth bar, at zero.
func (s *Store) HearthstonePlacementsFor(ctx context.Context, userID, gameID string) ([]HearthstonePlacementCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT placement, count(*)
		FROM hearthstone_matches
		WHERE user_id = $1 AND game_id = $2 AND placement IS NOT NULL
		GROUP BY placement
		ORDER BY placement`, userID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HearthstonePlacementCount
	for rows.Next() {
		var c HearthstonePlacementCount
		if err := rows.Scan(&c.Placement, &c.Matches); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// HearthstoneHeroesFor returns one member's record per Battlegrounds hero, most
// played first. Skins are folded onto the hero they dress, as everywhere else.
func (s *Store) HearthstoneHeroesFor(ctx context.Context, userID, gameID string) ([]HearthstoneHeroRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT regexp_replace(m.hero_card_id, '_SKIN_[A-Za-z0-9]+$', ''),
		       count(*),
		       avg(m.placement),
		       count(*) FILTER (WHERE m.placement <= 4),
		       count(*) FILTER (WHERE m.result = 'win')
		FROM hearthstone_matches m
		WHERE m.user_id = $1 AND m.game_id = $2
		  AND m.hero_card_id IS NOT NULL AND m.placement IS NOT NULL
		GROUP BY 1
		ORDER BY count(*) DESC, avg(m.placement) ASC`, userID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HearthstoneHeroRecord
	for rows.Next() {
		var r HearthstoneHeroRecord
		if err := rows.Scan(&r.HeroCardID, &r.Matches,
			&r.AvgPlacement, &r.Top4, &r.Wins); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// HearthstoneRecentFor returns a member's last matches, newest first.
//
// The limit serves two displays at once: the handful shown as a list, and the
// longer run the page averages into a trend.
func (s *Store) HearthstoneRecentFor(ctx context.Context, userID, gameID string, limit int) ([]HearthstoneMatchRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT played_at, mode, result, turns, placement, hero_card_id, rating, rating_after
		FROM hearthstone_matches
		WHERE user_id = $1 AND game_id = $2
		ORDER BY played_at DESC
		LIMIT $3`, userID, gameID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HearthstoneMatchRow
	for rows.Next() {
		var r HearthstoneMatchRow
		if err := rows.Scan(&r.PlayedAt, &r.Mode, &r.Result, &r.Turns,
			&r.Placement, &r.HeroCardID, &r.Rating, &r.RatingAfter); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// HearthstoneRating is a member's last known Battlegrounds rating and the
// instant of the match that produced it. The instant matters: a member who
// stopped using HDT keeps an old rating, and the page must say how old.
type HearthstoneRating struct {
	RatingAfter int
	At          time.Time
}

// HearthstoneLatestRatingFor returns the member's last known rating, or nil
// when no match of theirs ever carried one. Matches without a rating are
// skipped, never read as zero.
func (s *Store) HearthstoneLatestRatingFor(ctx context.Context, userID, gameID string) (*HearthstoneRating, error) {
	var r HearthstoneRating
	err := s.pool.QueryRow(ctx, `
		SELECT rating_after, played_at
		FROM hearthstone_matches
		WHERE user_id = $1 AND game_id = $2 AND rating_after IS NOT NULL
		ORDER BY played_at DESC
		LIMIT 1`, userID, gameID).Scan(&r.RatingAfter, &r.At)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// HearthstoneLatestRatingsByGame returns every member's last known rating for
// a game, keyed by user id. Members who never had one are absent. Banned
// members are excluded, as in every guild aggregate.
func (s *Store) HearthstoneLatestRatingsByGame(ctx context.Context, gameID string) (map[string]HearthstoneRating, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (m.user_id) m.user_id, m.rating_after, m.played_at
		FROM hearthstone_matches m
		JOIN users u ON u.id = m.user_id AND u.banned_at IS NULL
		WHERE m.game_id = $1 AND m.rating_after IS NOT NULL
		ORDER BY m.user_id, m.played_at DESC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]HearthstoneRating{}
	for rows.Next() {
		var id string
		var r HearthstoneRating
		if err := rows.Scan(&id, &r.RatingAfter, &r.At); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}
