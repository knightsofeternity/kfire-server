package store

import (
	"context"
	"time"
)

// RecapMatchOwner is the member a match CAME FROM, and nothing more.
//
// Nothing here infers a line-up from two members having played at the same
// minute: that would be a guess served as a fact. The only link between two
// members' matches is the match_key their clients computed from the game's own
// match identifier, which the API uses to group their reports.
type RecapMatchOwner struct {
	UserID    string
	Username  string
	AvatarURL *string
}

// RecapGame identifies the game a match belongs to, so a single timeline can
// mix games without the caller having to look each one up.
type RecapGame struct {
	GameID   string
	GameSlug string
	GameName string
	// GameIcon is the game's source icon URL, nil when the catalog has none.
	// The API never hands this out as is: it decides from nil whether to emit
	// a link to the image cache at all, so a game without an icon produces no
	// broken image rather than a 404 the page would have to hide.
	GameIcon *string
}

// RecapRocketLeagueMatch is one Rocket League match played inside a recap
// window. The statistics are the reporting member's own, as stored.
type RecapRocketLeagueMatch struct {
	RecapMatchOwner
	RecapGame
	// ID is the match row, so the page can ask for its scoreboard.
	ID string
	// MatchKey is shared by every member of the same match; nil for a report
	// without a scoreboard. The recap groups on it.
	MatchKey        *string
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
}

// RecapHearthstoneMatch is one Hearthstone match played inside a recap window.
type RecapHearthstoneMatch struct {
	RecapMatchOwner
	RecapGame
	Mode       string
	Result     string
	Turns      *int
	Placement  *int
	HeroCardID *string
	PlayedAt   time.Time
}

// RocketLeagueMatchesBetween returns every member's Rocket League matches in
// [from, to), oldest first: a recap is read in the order the evening happened.
//
// The window is half-open so two adjacent windows neither drop nor double a
// match played exactly on their shared bound.
//
// Banned members are excluded, as in every other aggregate of the portal.
// RecapViewer is who is asking for a recap, which decides whose matches they
// are allowed to see.
//
// A recap lists, minute by minute, what a member played during an evening.
// That is a listing of recent sessions in everything but name, so it honours
// the same toggle the sessions endpoint does: a member who turned
// sessions_visible off asked not to be shown, and 2 of the guild's 34 members
// actually have. Aggregates are filtered too, not just the timeline: saying
// "six matches this evening" would give away exactly what the toggle hides.
//
// A member always sees themselves, and an admin sees everyone, as elsewhere.
type RecapViewer struct {
	UserID  string
	IsAdmin bool
}

func (s *Store) RocketLeagueMatchesBetween(ctx context.Context, from, to time.Time, v RecapViewer) ([]RecapRocketLeagueMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.match_key,
		       m.user_id, u.username, u.avatar_url,
		       m.game_id, g.slug, g.name, g.icon_url,
		       m.playlist, m.team_size, m.player_team,
		       m.team_blue_score, m.team_orange_score, m.result,
		       m.goals, m.assists, m.saves, m.shots, m.score, m.demos,
		       m.mvp, m.duration_seconds, m.played_at
		FROM rocket_league_matches m
		JOIN users u ON u.id = m.user_id AND u.banned_at IS NULL
		                       AND (u.sessions_visible OR u.id = $3 OR $4)
		JOIN games g ON g.id = m.game_id
		WHERE m.played_at >= $1 AND m.played_at < $2
		ORDER BY m.played_at ASC, u.username ASC`, from, to, v.UserID, v.IsAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecapRocketLeagueMatch
	for rows.Next() {
		var m RecapRocketLeagueMatch
		if err := rows.Scan(&m.ID, &m.MatchKey,
			&m.UserID, &m.Username, &m.AvatarURL,
			&m.GameID, &m.GameSlug, &m.GameName, &m.GameIcon,
			&m.Playlist, &m.TeamSize, &m.PlayerTeam,
			&m.TeamBlueScore, &m.TeamOrangeScore, &m.Result,
			&m.Goals, &m.Assists, &m.Saves, &m.Shots, &m.Score, &m.Demos,
			&m.MVP, &m.DurationSeconds, &m.PlayedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// HearthstoneMatchesBetween returns every member's Hearthstone matches in
// [from, to), oldest first. Same window and same exclusions as its Rocket
// League counterpart.
func (s *Store) HearthstoneMatchesBetween(ctx context.Context, from, to time.Time, v RecapViewer) ([]RecapHearthstoneMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.user_id, u.username, u.avatar_url,
		       m.game_id, g.slug, g.name, g.icon_url,
		       m.mode, m.result, m.turns, m.placement, m.hero_card_id, m.played_at
		FROM hearthstone_matches m
		JOIN users u ON u.id = m.user_id AND u.banned_at IS NULL
		                       AND (u.sessions_visible OR u.id = $3 OR $4)
		JOIN games g ON g.id = m.game_id
		WHERE m.played_at >= $1 AND m.played_at < $2
		ORDER BY m.played_at ASC, u.username ASC`, from, to, v.UserID, v.IsAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecapHearthstoneMatch
	for rows.Next() {
		var m RecapHearthstoneMatch
		if err := rows.Scan(&m.UserID, &m.Username, &m.AvatarURL,
			&m.GameID, &m.GameSlug, &m.GameName, &m.GameIcon,
			&m.Mode, &m.Result, &m.Turns, &m.Placement, &m.HeroCardID,
			&m.PlayedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
