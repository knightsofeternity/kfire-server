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

// recentMatches is how many matches the member page asks for. Ten are listed;
// the rest feed the trend, which averages over twenty and needs a run longer
// than twenty to move.
const recentMatches = 40

// UserGameDetail returns the member's own record: totals, how often each place
// was reached, the heroes played, and the last matches.
//
// A member who has reported nothing yields nothing, and the page shows no block
// at all rather than a shelf of zeroes.
func (p *Plugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	totals, err := p.st.HearthstoneMemberTotalsFor(ctx, userID, g.ID)
	if err != nil {
		return nil, err
	}
	if totals.Matches == 0 {
		return nil, nil
	}

	places, err := p.st.HearthstonePlacementsFor(ctx, userID, g.ID)
	if err != nil {
		return nil, err
	}
	heroes, err := p.st.HearthstoneHeroesFor(ctx, userID, g.ID)
	if err != nil {
		return nil, err
	}
	recent, err := p.st.HearthstoneRecentFor(ctx, userID, g.ID, recentMatches)
	if err != nil {
		return nil, err
	}

	// Every list is materialised, never left nil: the browser iterates over
	// them and a missing key would read as an error rather than as nothing.
	byPlace := make([]map[string]any, len(places))
	for i, c := range places {
		byPlace[i] = map[string]any{"placement": c.Placement, "matches": c.Matches}
	}
	hs := make([]map[string]any, len(heroes))
	for i, h := range heroes {
		hs[i] = map[string]any{
			"hero_card_id": h.HeroCardID, "matches": h.Matches,
			"avg_placement": h.AvgPlacement, "top4": h.Top4, "wins": h.Wins,
		}
	}
	ms := make([]map[string]any, len(recent))
	for i, m := range recent {
		e := map[string]any{
			"played_at": m.PlayedAt, "mode": m.Mode, "result": m.Result,
		}
		if m.Turns != nil {
			e["turns"] = *m.Turns
		}
		if m.Placement != nil {
			e["placement"] = *m.Placement
		}
		if m.HeroCardID != nil {
			e["hero_card_id"] = *m.HeroCardID
		}
		ms[i] = e
	}

	profile := map[string]any{
		"matches": totals.Matches, "wins": totals.Wins, "ranked": totals.Ranked,
		"top4": totals.Top4, "by_placement": byPlace, "heroes": hs, "recent": ms,
	}
	if totals.AvgPlacement != nil {
		profile["avg_placement"] = *totals.AvgPlacement
	}
	return map[string]any{"hs_profile": profile}, nil
}
