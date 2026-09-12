package riotsync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// liveTTL bounds how long a live-game entry is trusted without a refresh. The
// poller runs well inside this window, so an entry only expires when the poller
// itself has stopped or is failing.
const liveTTL = 3 * time.Minute

// liveSlug is the catalog slug whose open sessions drive the poller.
const liveSlug = "league-of-legends"

type liveEntry struct {
	game     *riot.LiveGame
	storedAt time.Time
}

// liveRegistry holds who is in a League game right now. Nothing is persisted:
// the information is transient and rebuilds within one poll after a restart.
type liveRegistry struct {
	mu      sync.RWMutex
	entries map[string]liveEntry
}

func newLiveRegistry() *liveRegistry {
	return &liveRegistry{entries: map[string]liveEntry{}}
}

// set records a member's live game. A nil game clears the entry, so leaving a
// game shows up on the next poll rather than at TTL expiry.
func (r *liveRegistry) set(userID string, g *riot.LiveGame) {
	if g == nil {
		r.clear(userID)
		return
	}
	r.mu.Lock()
	r.entries[userID] = liveEntry{game: g, storedAt: time.Now()}
	r.mu.Unlock()
}

func (r *liveRegistry) clear(userID string) {
	r.mu.Lock()
	delete(r.entries, userID)
	r.mu.Unlock()
}

// get returns a member's live game, or nil when absent or stale.
func (r *liveRegistry) get(userID string) *riot.LiveGame {
	r.mu.RLock()
	e, ok := r.entries[userID]
	r.mu.RUnlock()
	if !ok || time.Since(e.storedAt) > liveTTL {
		return nil
	}
	return e.game
}

// LiveGame returns a member's match in progress, or nil.
func (s *Syncer) LiveGame(userID string) *riot.LiveGame {
	if s.live == nil {
		return nil
	}
	return s.live.get(userID)
}

// RunLive polls Spectator for the members who are currently in a League
// session, every interval, until ctx is done.
//
// The member list comes from open game sessions, so the poller costs nothing
// when nobody is playing, and one call per player per tick otherwise.
func (s *Syncer) RunLive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollLive(ctx)
		}
	}
}

// pollLive runs one round.
func (s *Syncer) pollLive(ctx context.Context) {
	if s.riot == nil || !s.riot.Enabled() {
		return
	}
	if !s.pluginActive() {
		return // the admin turned League off; stop calling Riot for it
	}
	game, err := s.store.GetGameBySlug(ctx, liveSlug)
	if err != nil {
		// League missing from this instance's catalog is a legitimate, quiet
		// state. A database failure is not, and would otherwise vanish on
		// every tick.
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("riotsync: get game", "slug", liveSlug, "err", err)
		}
		return
	}
	players, err := s.store.RiotPlayersInGame(ctx, game.ID)
	if err != nil {
		slog.Warn("riotsync: live players", "err", err)
		return
	}
	for _, p := range players {
		live, err := s.riot.ActiveGame(ctx, p.Platform, p.PUUID)
		if err != nil {
			// A 404 is already folded into (nil, nil) by ActiveGame, so
			// anything here is a real failure worth seeing.
			slog.Warn("riotsync: active game", "user_id", p.UserID, "err", err)
			continue
		}
		s.live.set(p.UserID, live)
	}
}
