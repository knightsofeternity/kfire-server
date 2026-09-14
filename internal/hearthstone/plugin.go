// Package hearthstone exposes Hearthstone as a game plugin. Unlike the other
// plugins it crawls nothing: match results are reported by the desktop client,
// which reads the game's own log files.
package hearthstone

import (
	"context"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// Plugin shows the guild's Hearthstone record.
type Plugin struct {
	st *store.Store
}

// New builds the plugin.
func New(st *store.Store) *Plugin { return &Plugin{st: st} }

func (p *Plugin) ID() string      { return "hearthstone" }
func (p *Plugin) Name() string    { return "Hearthstone" }
func (p *Plugin) Slugs() []string { return []string{"hearthstone"} }

// Connector returns an empty string: Hearthstone has no credential layer.
func (p *Plugin) Connector() string { return "" }

// Available is always true. Every other plugin gates on its connector holding a
// key; this one has no key to hold, because the data arrives from the member's
// own machine. The admin switch in the plugin registry remains the way to turn
// it off.
func (p *Plugin) Available() bool { return true }

// Refresh does nothing: there is nothing to crawl. Results arrive over the
// control plane when a member finishes a match.
func (p *Plugin) Refresh(ctx context.Context, userID, gameSlug string) {}

// GameDetail returns the guild's record, one entry per member.
func (p *Plugin) GameDetail(ctx context.Context, _ string, g store.Game) (map[string]any, error) {
	stats, err := p.st.HearthstoneStatsByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	cards := make([]map[string]any, len(stats))
	for i, s := range stats {
		m := map[string]any{
			"user_id": s.UserID, "username": s.Username,
			"matches": s.Matches, "wins": s.Wins,
			"ranked": s.Ranked, "top4": s.Top4,
			"last_played_at": s.LastPlayedAt,
		}
		if s.AvatarURL != nil {
			m["avatar_url"] = *s.AvatarURL
		}
		if s.AvgPlacement != nil {
			m["avg_placement"] = *s.AvgPlacement
		}
		cards[i] = m
	}
	heroes, err := p.st.HearthstoneHeroesByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	hs := make([]map[string]any, len(heroes))
	for i, h := range heroes {
		hs[i] = map[string]any{
			"hero_card_id": h.HeroCardID, "matches": h.Matches,
			"players": h.Players, "avg_placement": h.AvgPlacement,
			"top4": h.Top4, "wins": h.Wins,
		}
	}

	// The hero's name is NOT resolved here. The log writes a card identifier,
	// the browser turns it into a name and an illustration: the identifier is
	// the fact, the name is presentation, and it differs per language.
	return map[string]any{"hs_players": cards, "hs_heroes": hs}, nil
}

// UserGameDetail returns nothing for now: the member page keeps its current
// display, which is out of this chantier's scope.
func (p *Plugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	return nil, nil
}
