// Package xboxsync polls OpenXBL presence for linked members and reconciles it
// into xbox_api game sessions, broadcasting presence so consoles show live.
package xboxsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/xbox"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

type Syncer struct {
	store  *store.Store
	xbl    *xbox.Connector
	cipher *crypto.Cipher
	hub    *ws.Hub
}

func New(st *store.Store, xbl *xbox.Connector, cipher *crypto.Cipher, hub *ws.Hub) *Syncer {
	return &Syncer{store: st, xbl: xbl, cipher: cipher, hub: hub}
}

// reconcileUser fetches one member's presence and opens/keeps/closes their
// xbox_api session, returning whether presence changed.
func (s *Syncer) reconcileUser(ctx context.Context, userID, xuid string, tokenEnc []byte) bool {
	token, err := s.cipher.OpenString(tokenEnc)
	if err != nil {
		return false
	}
	p, err := s.xbl.Presence(ctx, token, xuid)
	if err != nil || p == nil {
		slog.Warn("xboxsync: presence", "user_id", userID, "err", err)
		return false
	}
	open, err := s.store.OpenSessionBySource(ctx, userID, "xbox_api")
	if err != nil {
		return false
	}
	if !p.Playing {
		if open != nil {
			changed, _ := s.store.EndSession(ctx, userID, open.Game.ID)
			return changed
		}
		return false
	}
	game, err := s.store.UpsertXboxGame(ctx, p.TitleID, p.TitleName)
	if err != nil {
		slog.Error("xboxsync: resolve game", "title", p.TitleName, "err", err)
		return false
	}
	if open != nil && open.Game.ID == game.ID {
		return false
	}
	if open != nil {
		// Close the previous game first; if that fails, abort rather than open a
		// second session and leave a zombie open xbox_api session behind.
		if _, err := s.store.EndSession(ctx, userID, open.Game.ID); err != nil {
			slog.Error("xboxsync: end previous session", "user_id", userID, "err", err)
			return false
		}
	}
	changed, err := s.store.StartSession(ctx, userID, game.ID, "xbox_api")
	if err != nil {
		slog.Error("xboxsync: start session", "user_id", userID, "err", err)
		return false
	}
	return changed
}

// SyncAll reconciles every xbox-linked member, broadcasting presence on change.
func (s *Syncer) SyncAll(ctx context.Context) {
	users, err := s.store.ListLinkedTokensByProvider(ctx, "xbox")
	if err != nil {
		slog.Error("xboxsync: list linked", "err", err)
		return
	}
	for _, u := range users {
		if s.reconcileUser(ctx, u.UserID, u.ProviderUserID, u.AccessTokenEnc) {
			if full, err := s.store.GetUserByID(ctx, u.UserID); err == nil {
				s.hub.BroadcastPresence(ctx, ws.PresenceUser{
					ID: full.ID, Username: full.Username, AvatarURL: full.AvatarURL,
					ActivityVisible: full.ActivityVisible,
				})
			}
		}
	}
}

func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	slog.Info("xboxsync: poller started", "interval", interval)
	t := time.NewTimer(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.SyncAll(ctx)
			t.Reset(interval)
		}
	}
}
