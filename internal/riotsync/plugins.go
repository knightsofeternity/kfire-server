package riotsync

import (
	"context"
	"encoding/json"
	"slices"
	"sort"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// LolPlugin exposes League of Legends as a game plugin, wrapping the Riot
// syncer and the store reads.
//
// It lives in riotsync rather than in the connector package for the same reason
// the Battle.net plugins do: the connector must not import the syncer.
type LolPlugin struct {
	st     *store.Store
	syncer *Syncer
	conn   *riot.Connector
}

// NewLolPlugin builds the League plugin.
func NewLolPlugin(st *store.Store, s *Syncer, conn *riot.Connector) *LolPlugin {
	return &LolPlugin{st: st, syncer: s, conn: conn}
}

func (p *LolPlugin) ID() string        { return "lol" }
func (p *LolPlugin) Name() string      { return "League of Legends" }
func (p *LolPlugin) Connector() string { return "riot" }
func (p *LolPlugin) Available() bool   { return p.conn.Enabled() }
func (p *LolPlugin) Slugs() []string   { return []string{liveSlug} }

func (p *LolPlugin) Refresh(ctx context.Context, userID, gameSlug string) {
	if !slices.Contains(p.Slugs(), gameSlug) {
		return
	}
	g, err := p.st.GetGameBySlug(ctx, gameSlug)
	if err != nil {
		return
	}
	p.syncer.RefreshLoL(ctx, userID, g.ID)
}

// scoreOf reads the precomputed solo-queue score out of a stored blob. A blob
// that cannot be read sorts last rather than failing the page. The field is a
// pointer so that a real score of zero, iron IV at 0 LP, is not mistaken for
// the unranked sentinel.
func scoreOf(blob []byte) int {
	var p struct {
		SoloScore *int `json:"solo_score"`
	}
	if err := json.Unmarshal(blob, &p); err != nil || p.SoloScore == nil {
		return -1
	}
	return *p.SoloScore
}

// sortByScore orders cards by solo-queue standing, highest first, then by
// username so unranked members keep a stable alphabetical order.
func sortByScore(cards []map[string]any) {
	sort.SliceStable(cards, func(i, j int) bool {
		si, _ := cards[i]["solo_score"].(float64)
		sj, _ := cards[j]["solo_score"].(float64)
		if si != sj {
			return si > sj
		}
		ui, _ := cards[i]["username"].(string)
		uj, _ := cards[j]["username"].(string)
		return ui < uj
	})
}

// GameDetail returns the guild leaderboard for the game page, ranked members
// first.
func (p *LolPlugin) GameDetail(ctx context.Context, _ string, g store.Game) (map[string]any, error) {
	profs, synced, err := p.st.RiotProfilesByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	cards := make([]map[string]any, len(profs))
	for i, pr := range profs {
		card := map[string]any{
			"user_id":    pr.UserID,
			"username":   pr.Username,
			"solo_score": float64(scoreOf(pr.Data)),
			"data":       json.RawMessage(pr.Data),
		}
		if pr.AvatarURL != nil {
			card["avatar_url"] = *pr.AvatarURL
		}
		if live := p.syncer.LiveGame(pr.UserID); live != nil {
			card["live"] = live
		}
		cards[i] = card
	}
	sortByScore(cards)
	return map[string]any{"lol_players": cards, "lol_synced_at": synced}, nil
}

// UserGameDetail returns one member's League card.
func (p *LolPlugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	data, err := p.st.RiotProfileForUserGame(ctx, userID, g.ID)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	out := map[string]any{"lol_profile": json.RawMessage(data)}
	if live := p.syncer.LiveGame(userID); live != nil {
		out["lol_live"] = live
	}
	return out, nil
}
