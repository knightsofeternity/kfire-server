package rocketleague

import (
	"context"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// recentMatches is the number of matches requested by the member page.
const recentMatches = 10

// Plugin shows the guild's Rocket League record.
type Plugin struct {
	st *store.Store
}

// New builds the plugin.
func New(st *store.Store) *Plugin { return &Plugin{st: st} }

func (p *Plugin) ID() string      { return "rocket-league" }
func (p *Plugin) Name() string    { return "Rocket League" }
func (p *Plugin) Slugs() []string { return []string{"rocket-league"} }

// Connector returns an empty string: Rocket League has no credential layer.
// Same situation as Hearthstone.
func (p *Plugin) Connector() string { return "" }

// Available is always true. Every other plugin depends on a connector
// holding a key; this one has no key to hold, because the data arrives from
// the member's own machine. The admin switch in the registry remains the way
// to turn it off.
func (p *Plugin) Available() bool { return true }

// Refresh does nothing: there is nothing to crawl. Results arrive over the
// control plane when a member finishes a match.
func (p *Plugin) Refresh(ctx context.Context, userID, gameSlug string) {}

// GameDetail returns the guild's record, one entry per member.
func (p *Plugin) GameDetail(ctx context.Context, _ string, g store.Game) (map[string]any, error) {
	stats, err := p.st.RocketLeagueStatsByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	cards := make([]map[string]any, len(stats))
	for i, s := range stats {
		m := map[string]any{
			"user_id": s.UserID, "username": s.Username,
			"matches": s.Matches, "wins": s.Wins, "mvps": s.MVPs,
			"goals": s.Goals, "assists": s.Assists, "saves": s.Saves,
			"shots": s.Shots, "demos": s.Demos, "score": s.Score,
			"play_time_seconds": s.PlayTime,
			"last_played_at":    s.LastPlayedAt,
		}
		if s.AvatarURL != nil {
			m["avatar_url"] = *s.AvatarURL
		}
		cards[i] = m
	}
	return map[string]any{"rl_players": cards}, nil
}

// UserGameDetail returns a member's block: their latest matches.
//
// The playlist identifier is NOT translated here. The browser turns it into
// a label, because the identifier is the fact and the label is presentation,
// different per language. Same choice as hero_card_id for Hearthstone.
func (p *Plugin) UserGameDetail(ctx context.Context, targetUserID string, g store.Game) (map[string]any, error) {
	matches, err := p.st.RocketLeagueRecentMatches(ctx, targetUserID, g.ID, recentMatches)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{
			"playlist": m.Playlist, "team_size": m.TeamSize,
			"player_team":     m.PlayerTeam,
			"team_blue_score": m.TeamBlueScore, "team_orange_score": m.TeamOrangeScore,
			"result": m.Result, "goals": m.Goals, "assists": m.Assists,
			"saves": m.Saves, "shots": m.Shots, "score": m.Score, "demos": m.Demos,
			"mvp": m.MVP, "duration_seconds": m.DurationSeconds,
			"played_at": m.PlayedAt,
		}
	}
	return map[string]any{"rl_matches": out}, nil
}
