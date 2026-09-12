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

// showLive decides whether a match in progress may be revealed. A member's
// standing is public either way: hiding your activity hides WHAT you are
// playing, not THAT you play. Members always see their own.
func showLive(activityVisible bool, memberID, viewerID string) bool {
	return activityVisible || (viewerID != "" && memberID == viewerID)
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
//
// The match in progress is the one piece here that reveals what someone is
// doing right now, so it honours the member's activity toggle exactly as the
// generic presence does. Their standing stays visible either way: hiding your
// activity hides what you are playing, not that you play.
func (p *LolPlugin) GameDetail(ctx context.Context, viewerID string, g store.Game) (map[string]any, error) {
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
		if showLive(pr.ActivityVisible, pr.UserID, viewerID) {
			if live := p.syncer.LiveGame(pr.UserID); live != nil {
				card["live"] = live
			}
		}
		cards[i] = card
	}
	sortByScore(cards)
	return map[string]any{"lol_players": cards, "lol_synced_at": synced}, nil
}

// UserGameDetail returns one member's League card.
//
// This surface is also served to unauthenticated third parties through the
// public API, and the interface carries no viewer, so the match in progress is
// gated on the member's activity toggle alone. A member who hides their
// activity therefore does not see their own live block here either, which is
// the safe way round: the alternative would leak it to everyone.
func (p *LolPlugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	data, err := p.st.RiotProfileForUserGame(ctx, userID, g.ID)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	out := map[string]any{"lol_profile": json.RawMessage(data)}
	// No viewer on this interface, so the self exception cannot apply here.
	if u, err := p.st.GetUserByID(ctx, userID); err == nil && showLive(u.ActivityVisible, userID, "") {
		if live := p.syncer.LiveGame(userID); live != nil {
			out["lol_live"] = live
		}
	}
	return out, nil
}
