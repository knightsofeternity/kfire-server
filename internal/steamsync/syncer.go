// Package steamsync imports Steam library playtime and achievements for linked
// members, on a background schedule and on demand.
package steamsync

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/steam"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// maxAchievementGames bounds how many of a member's games we pull achievements
// for, to keep the number of Steam API calls reasonable. Games without
// achievement progress simply return empty and are skipped.
const maxAchievementGames = 30

// Result summarizes one user sync.
type Result struct {
	GamesImported        int `json:"games_imported"`
	AchievementsImported int `json:"achievements_imported"`
}

// Syncer imports Steam data into the store.
type Syncer struct {
	store *store.Store
	steam *steam.Connector
}

func New(st *store.Store, conn *steam.Connector) *Syncer {
	return &Syncer{store: st, steam: conn}
}

// SyncUser imports one member's Steam library and achievements.
func (s *Syncer) SyncUser(ctx context.Context, userID, steamID string) (Result, error) {
	var res Result

	owned, err := s.steam.GetOwnedGames(ctx, steamID)
	if err != nil {
		return res, err
	}
	if len(owned) == 0 {
		return res, nil // private profile or empty library
	}

	appIDs := make([]string, len(owned))
	for i, g := range owned {
		appIDs[i] = g.AppID
	}
	catalog, err := s.store.GamesBySteamAppID(ctx, appIDs)
	if err != nil {
		return res, err
	}

	// Import playtime for catalog-matched games, remembering the matches so we
	// can pick the most-played ones for achievements.
	type match struct {
		game     store.Game
		appID    string
		playtime time.Duration
	}
	var matched []match
	for _, g := range owned {
		game, ok := catalog[g.AppID]
		if !ok {
			// Not matched by AppID. The Discord catalog can carry a wrong or
			// missing Steam AppID (e.g. Rocket League), so resolve the game from
			// the Steam library itself: match by name and correct the AppID, or
			// create a new imported entry. This imports the player's whole library.
			game, err = s.store.UpsertSteamGame(ctx, g.AppID, g.Name, g.IconURL)
			if err != nil {
				return res, err
			}
		}
		secs := int64(g.PlaytimeForever.Seconds())
		if err := s.store.UpsertExternalPlaytime(ctx, userID, "steam", game.ID, secs); err != nil {
			return res, err
		}
		res.GamesImported++
		matched = append(matched, match{game: game, appID: g.AppID, playtime: g.PlaytimeForever})
	}

	// Achievements: try the most-played games first; this still works when the
	// player keeps total playtime private (all zero), bounded by
	// maxAchievementGames so the number of Steam API calls stays reasonable.
	sort.Slice(matched, func(i, j int) bool { return matched[i].playtime > matched[j].playtime })
	if len(matched) > maxAchievementGames {
		matched = matched[:maxAchievementGames]
	}

	for _, m := range matched {
		unlocked, err := s.steam.GetPlayerAchievements(ctx, steamID, m.appID)
		if err != nil || len(unlocked) == 0 {
			continue
		}
		schema, _ := s.steam.GetGameSchema(ctx, m.appID)

		rows := make([]store.AchievementRow, 0, len(unlocked))
		for _, a := range unlocked {
			meta := schema[a.APIName]
			rows = append(rows, store.AchievementRow{
				GameID:      m.game.ID,
				APIName:     a.APIName,
				DisplayName: meta.DisplayName,
				IconURL:     meta.IconURL,
				UnlockedAt:  a.UnlockedAt,
			})
		}
		if err := s.store.UpsertAchievements(ctx, userID, "steam", rows); err != nil {
			return res, err
		}
		res.AchievementsImported += len(rows)
	}

	return res, nil
}

// SyncAll syncs every linked Steam account. Per-user errors are logged and
// skipped so one bad account doesn't stall the rest.
func (s *Syncer) SyncAll(ctx context.Context) {
	users, err := s.store.ListLinkedByProvider(ctx, "steam")
	if err != nil {
		slog.Error("steamsync: list linked accounts", "err", err)
		return
	}
	for _, u := range users {
		res, err := s.SyncUser(ctx, u.UserID, u.ProviderUserID)
		if err != nil {
			slog.Error("steamsync: user sync failed", "user_id", u.UserID, "err", err)
			continue
		}
		slog.Info("steamsync: user synced", "user_id", u.UserID,
			"games", res.GamesImported, "achievements", res.AchievementsImported)
	}
}

// Run periodically syncs all linked accounts until the context is cancelled.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	slog.Info("steamsync: poller started", "interval", interval)
	// First pass shortly after boot, then on the interval.
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.SyncAll(ctx)
			timer.Reset(interval)
		}
	}
}
