package nintendosync

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/nintendo"
	"github.com/knightsofeternity/kfire-server/internal/gametitle"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

const (
	libraryEvery  = 6 * time.Hour
	sessionSource = "nintendo_api"
	provider      = "nintendo"
)

// PresenceFunc turns a user into the hub's presence payload; injected by the
// api package (presenceUser), which applies the member's chosen status.
type PresenceFunc func(ctx context.Context, userID string) (ws.PresenceUser, bool)

// Syncer runs every request one after the other: the nxapi terms allow a
// single automated request at a time, and never an automatic retry.
type Syncer struct {
	st       *store.Store
	bot      *Bot
	hub      *ws.Hub
	presence PresenceFunc
}

func New(st *store.Store, bot *Bot, hub *ws.Hub) *Syncer {
	return &Syncer{st: st, bot: bot, hub: hub}
}

func (s *Syncer) Bot() *Bot                      { return s.bot }
func (s *Syncer) SetPresenceFunc(f PresenceFunc) { s.presence = f }

// toAccept keeps the pending requests sent by linked members.
func toAccept(reqs []nintendo.FriendRequest, linked map[string]string) []nintendo.FriendRequest {
	var out []nintendo.FriendRequest
	for _, r := range reqs {
		if _, ok := linked[r.Sender.NsaID]; ok {
			out = append(out, r)
		}
	}
	return out
}

// linkedMembers maps Nintendo nsaId to KFIRE user id.
func (s *Syncer) linkedMembers(ctx context.Context) (map[string]string, error) {
	users, err := s.st.ListLinkedByProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.ProviderUserID] = u.UserID
	}
	return m, nil
}

// failed reports whether a pass must stop, marking the bot when Nintendo
// refused its session.
func (s *Syncer) failed(ctx context.Context, what string, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, nintendo.ErrSessionRevoked) {
		s.bot.Revoked(ctx)
	} else {
		slog.Warn("nintendosync: "+what, "err", err)
	}
	return true
}

// Tick runs one pass: accept linked members' requests, reconcile presence,
// import due play logs.
func (s *Syncer) Tick(ctx context.Context) {
	session, err := s.bot.Session(ctx)
	if errors.Is(err, ErrNeedsLogin) {
		s.closeSessions(ctx, nil) // blind until an admin logs the bot in
		return
	}
	if err != nil {
		return
	}
	linked, err := s.linkedMembers(ctx)
	if err != nil || len(linked) == 0 {
		return
	}
	client := s.bot.Client()

	reqs, err := client.ReceivedRequests(ctx, session)
	if s.failed(ctx, "friend requests", err) {
		return
	}
	for _, r := range toAccept(reqs, linked) {
		if err := client.Accept(ctx, session, r.ID); s.failed(ctx, "accept", err) {
			return
		}
		slog.Info("nintendosync: friend request accepted", "user_id", linked[r.Sender.NsaID])
	}

	friends, err := client.Friends(ctx, session)
	if s.failed(ctx, "friends", err) {
		return
	}
	_ = s.st.MarkNintendoOK(ctx)
	isFriend := map[string]bool{}
	for _, f := range friends {
		userID, ok := linked[f.NsaID]
		if !ok {
			continue
		}
		isFriend[f.NsaID] = true
		if s.reconcile(ctx, userID, f) {
			s.broadcast(ctx, userID)
		}
	}
	s.closeSessions(ctx, func(nsaID string) bool { return !isFriend[nsaID] })

	for nsaID := range isFriend {
		userID := linked[nsaID]
		due, err := s.st.NintendoLibraryDue(ctx, userID, libraryEvery)
		if err != nil || !due {
			continue
		}
		if err := s.SyncLibrary(ctx, userID, nsaID); err != nil {
			if s.failed(ctx, "play log", err) {
				return
			}
		}
	}
}

func (s *Syncer) broadcast(ctx context.Context, userID string) {
	if s.presence == nil {
		return
	}
	if pu, ok := s.presence(ctx, userID); ok {
		s.hub.BroadcastPresence(ctx, pu)
	}
}

// closeSessions ends the nintendo_api session of every linked member for
// whom which returns true (all of them when which is nil).
func (s *Syncer) closeSessions(ctx context.Context, which func(nsaID string) bool) {
	linked, err := s.linkedMembers(ctx)
	if err != nil {
		return
	}
	for nsaID, userID := range linked {
		if which != nil && !which(nsaID) {
			continue
		}
		open, err := s.st.OpenSessionBySource(ctx, userID, sessionSource)
		if err != nil || open == nil {
			continue
		}
		if changed, _ := s.st.EndSession(ctx, userID, open.Game.ID); changed {
			s.broadcast(ctx, userID)
		}
	}
}

// reconcile opens, keeps or closes the member's nintendo_api session.
func (s *Syncer) reconcile(ctx context.Context, userID string, f nintendo.Friend) bool {
	open, err := s.st.OpenSessionBySource(ctx, userID, sessionSource)
	if err != nil {
		return false
	}
	if !f.Playing() {
		if open != nil {
			changed, _ := s.st.EndSession(ctx, userID, open.Game.ID)
			return changed
		}
		return false
	}
	g := f.Presence.Game
	game, err := s.st.UpsertNintendoGame(ctx, nintendo.TitleID(g.ShopURI), gametitle.Normalize(g.Name), g.ImageURI)
	if err != nil {
		slog.Error("nintendosync: resolve game", "title", g.Name, "err", err)
		return false
	}
	if open != nil && open.Game.ID == game.ID {
		return false
	}
	if open != nil {
		if _, err := s.st.EndSession(ctx, userID, open.Game.ID); err != nil {
			slog.Error("nintendosync: end previous session", "user_id", userID, "err", err)
			return false
		}
	}
	changed, err := s.st.StartSession(ctx, userID, game.ID, sessionSource)
	if err != nil {
		slog.Error("nintendosync: start session", "user_id", userID, "err", err)
		return false
	}
	return changed
}

// SyncLibrary imports a member's play history: every game with its total
// time, Switch 1 and Switch 2 editions of one game adding up.
func (s *Syncer) SyncLibrary(ctx context.Context, userID, nsaID string) error {
	session, err := s.bot.Session(ctx)
	if err != nil {
		return err
	}
	games, err := s.bot.Client().PlayLog(ctx, session, nsaID)
	if err != nil {
		return err
	}
	seconds := map[string]int64{}
	for _, g := range games {
		cg, err := s.st.UpsertNintendoGame(ctx, nintendo.TitleID(g.ShopURI), gametitle.Normalize(g.Name), g.ImageURI)
		if err != nil {
			slog.Warn("nintendosync: game", "name", g.Name, "err", err)
			continue
		}
		seconds[cg.ID] += g.TotalPlayTime * 60
	}
	for gameID, secs := range seconds {
		if err := s.st.UpsertExternalPlaytime(ctx, userID, provider, gameID, secs); err != nil {
			return err
		}
	}
	return s.st.MarkNintendoLibrarySynced(ctx, userID)
}

// Run polls every interval until ctx ends.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	slog.Info("nintendosync: poller started", "interval", interval)
	t := time.NewTimer(45 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
			t.Reset(interval)
		}
	}
}
