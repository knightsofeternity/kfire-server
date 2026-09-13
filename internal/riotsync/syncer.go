package riotsync

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// throttle bounds how often a member's League data is refreshed from Riot.
const throttle = time.Hour

// recentMatchCount is how many recent matches the card shows. Each one costs a
// detail call, so this number is the bulk of a refresh's cost.
const recentMatchCount = 5

// topChampionCount is how many mastery champions the card shows.
const topChampionCount = 3

// Syncer refreshes League data lazily, on view, and never in the background.
type Syncer struct {
	store  *store.Store
	riot   *riot.Connector
	dd     *riot.DataDragon
	live   *liveRegistry
	active func() bool
}

// New returns a syncer.
func New(st *store.Store, conn *riot.Connector, dd *riot.DataDragon) *Syncer {
	return &Syncer{store: st, riot: conn, dd: dd, live: newLiveRegistry()}
}

// SetActiveCheck tells the syncer how to ask whether the League plugin is still
// enabled by the admin. Without it the live-game loop would keep calling Riot
// every minute for a plugin the admin has turned off: the display surfaces are
// gated by the plugin registry, but a background loop is not.
//
// Call it once at wiring time, before the loop starts. The registry cannot be
// passed to New because it is built after the syncer it registers.
func (s *Syncer) SetActiveCheck(fn func() bool) { s.active = fn }

// pluginActive reports whether the League plugin is enabled. A syncer wired
// without a check is treated as active, so the lazy refresh path keeps working
// in tests and in any caller that does not own a registry.
func (s *Syncer) pluginActive() bool { return s.active == nil || s.active() }

// profile is the stored blob's shape. It is the contract with the SPA.
type profile struct {
	RiotID    string                 `json:"riot_id"`
	Platform  string                 `json:"platform"`
	SoloScore int                    `json:"solo_score"`
	Ranks     []riot.RankEntry       `json:"ranks"`
	Champions []riot.ChampionMastery `json:"top_champions"`
	Recent    []riot.MatchResult     `json:"recent"`
}

// buildProfile assembles the stored blob. Empty slices are materialised so the
// JSON carries [] rather than null and the SPA can iterate without a guard.
func buildProfile(riotID, platform string, ranks []riot.RankEntry,
	champions []riot.ChampionMastery, recent []riot.MatchResult) []byte {

	if ranks == nil {
		ranks = []riot.RankEntry{}
	}
	if champions == nil {
		champions = []riot.ChampionMastery{}
	}
	if recent == nil {
		recent = []riot.MatchResult{}
	}
	blob, err := json.Marshal(profile{
		RiotID: riotID, Platform: platform, SoloScore: SoloScore(ranks),
		Ranks: ranks, Champions: champions, Recent: recent,
	})
	if err != nil {
		// profile holds only plain types; marshalling cannot fail. Return an
		// empty object rather than panicking in a request path.
		return []byte(`{}`)
	}
	return blob
}

// RefreshLoL refreshes one member's League data if the throttle window has
// elapsed. Safe to call on every page view.
//
// Cost when it does run: eight calls. Ranks, mastery, the match id list, then
// one detail per match, the details in parallel.
func (s *Syncer) RefreshLoL(ctx context.Context, userID, gameID string) {
	if s.riot == nil || !s.riot.Enabled() {
		return // connector not configured on this instance
	}
	acc, err := s.store.RiotAccountFor(ctx, userID)
	if err != nil {
		// Not linked is the common, expected case and stays quiet; anything
		// else is a real database failure and must be visible.
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("riotsync: read riot account", "user_id", userID, "err", err)
		}
		return
	}
	synced, err := s.store.RiotProfileSyncedAt(ctx, userID, gameID)
	if err != nil {
		slog.Warn("riotsync: read sync time", "user_id", userID, "err", err)
		return // fail closed: with the database down, eight Riot calls would
		// only end in a failed write
	}
	if time.Since(synced) < throttle {
		return
	}

	ranks, err := s.riot.LeagueEntries(ctx, acc.Platform, acc.PUUID)
	if err != nil {
		// Nothing is written, so last_synced_at stays put and the next view
		// retries. A 429 lands here and simply backs off.
		slog.Warn("riotsync: league entries", "user_id", userID, "err", err)
		return
	}
	champions, err := s.riot.TopChampions(ctx, acc.Platform, acc.PUUID, topChampionCount)
	if err != nil {
		slog.Warn("riotsync: champion mastery", "user_id", userID, "err", err)
		return
	}
	for i := range champions {
		name, icon, imageID, err := s.dd.Champion(ctx, champions[i].ChampionID)
		if err != nil {
			continue // Data Dragon down: the SPA falls back to the bare id
		}
		champions[i].Name, champions[i].IconURL = name, icon
		champions[i].ImageID = imageID
	}
	// RecentMatches only errors on the match-id listing; a detail that fails is
	// skipped inside it and merely shortens the list. So an error here is one of
	// the three fatal calls: writing now would replace a good recent-form block
	// with an empty one and then block the retry for a whole hour.
	recent, err := s.riot.RecentMatches(ctx, acc.MatchCluster, acc.PUUID, recentMatchCount)
	if err != nil {
		slog.Warn("riotsync: recent matches", "user_id", userID, "err", err)
		return
	}

	blob := buildProfile(acc.RiotID, acc.Platform, ranks, champions, recent)
	if err := s.store.UpsertRiotProfile(ctx, userID, gameID, blob); err != nil {
		slog.Error("riotsync: store profile", "user_id", userID, "err", err)
	}
}
