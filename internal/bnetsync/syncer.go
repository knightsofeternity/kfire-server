// Package bnetsync refreshes a member's Battle.net WoW characters on demand.
// Battle.net issues no refresh token (24h access tokens), so we refresh only
// while the stored token is still valid and throttle by last sync.
package bnetsync

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/battlenet"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// throttle bounds how often a member's WoW data is refreshed from Blizzard.
const throttle = time.Hour

// wowNamespace pairs a catalog slug with its Blizzard profile namespace.
type wowNamespace struct {
	slug      string
	namespace string
}

type Syncer struct {
	store  *store.Store
	bnet   *battlenet.Connector
	cipher *crypto.Cipher
	region string
}

func New(st *store.Store, bnet *battlenet.Connector, cipher *crypto.Cipher, region string) *Syncer {
	return &Syncer{store: st, bnet: bnet, cipher: cipher, region: region}
}

// namespaces returns the retail and classic namespaces for the configured
// region, paired with their catalog slugs.
func (s *Syncer) namespaces() []wowNamespace {
	return []wowNamespace{
		{slug: "world-of-warcraft", namespace: "profile-" + s.region},
		{slug: "world-of-warcraft-classic", namespace: "profile-classic1x-" + s.region},
		{slug: "world-of-warcraft-classic", namespace: "profile-classic-" + s.region},
	}
}

// RefreshWoW refreshes one member's WoW characters for the given game slug if
// their token is fresh and the throttle window has elapsed. Safe to call on
// every page view.
func (s *Syncer) RefreshWoW(ctx context.Context, userID, gameSlug string) {
	if s.bnet == nil || !s.bnet.Enabled() {
		return // connector not configured on this instance
	}
	tok, err := s.store.GetLinkedToken(ctx, userID, "battlenet")
	if err != nil {
		return // not linked, or no token
	}
	if tok.TokenExpiresAt == nil || tok.TokenExpiresAt.Before(time.Now()) {
		return // token expired; UI shows the reconnect banner
	}
	if !hasScope(tok.Scopes, "wow.profile") {
		return
	}

	game, err := s.store.GetGameBySlug(ctx, gameSlug)
	if err != nil {
		return
	}
	// Throttle per user+game: skip if this member refreshed recently.
	if synced, err := s.store.WowSyncedAt(ctx, userID, game.ID); err == nil &&
		!synced.IsZero() && time.Since(synced) < throttle {
		return
	}

	access, err := s.cipher.OpenString(tok.AccessTokenEnc)
	if err != nil {
		slog.Warn("bnetsync: decrypt token (wow)", "user_id", userID, "err", err)
		return
	}

	// A catalog slug can map to several Blizzard namespaces (Classic has two:
	// classic1x and classic). Gather characters from all of them, then replace
	// the game's set ONCE so the namespaces don't clobber each other. Only
	// replace if at least one namespace fetch succeeded, so a transient API
	// error doesn't wipe previously-synced characters.
	var rows []store.WowCharacterRow
	anyOK := false
	for _, ns := range s.namespaces() {
		if ns.slug != gameSlug {
			continue
		}
		chars, err := s.bnet.WowAccountCharacters(ctx, access, ns.namespace)
		if err != nil {
			slog.Warn("bnetsync: wow account", "user_id", userID, "ns", ns.namespace, "err", err)
			continue
		}
		slog.Info("bnetsync: wow fetched", "user_id", userID, "ns", ns.namespace, "count", len(chars))
		anyOK = true
		for i := range chars {
			if err := s.bnet.EnrichWowCharacter(ctx, access, ns.namespace, &chars[i]); err != nil {
				slog.Warn("bnetsync: enrich", "char", chars[i].Name, "err", err)
				continue
			}
			achs, err := s.bnet.WowAchievements(ctx, access, ns.namespace, chars[i].RealmSlug, chars[i].Name)
			if err != nil {
				slog.Warn("bnetsync: wow achievements", "char", chars[i].Name, "err", err)
			}
			var achJSON []byte
			if len(achs) > 0 {
				achJSON, _ = json.Marshal(achs)
			}
			c := chars[i]
			rows = append(rows, store.WowCharacterRow{
				Region: s.region, RealmSlug: c.RealmSlug, Name: c.Name, RealmName: c.RealmName,
				Faction: strPtr(c.Faction), Race: strPtr(c.Race), Class: strPtr(c.Class),
				Level: c.Level, ItemLevel: c.ItemLevel, MythicRating: c.MythicRating,
				RaidSummary: rawOrNil(c.RaidSummary), AchievementPoints: c.AchievementPoints,
				Achievements: achJSON,
			})
		}
	}
	if anyOK {
		if err := s.store.ReplaceWowCharacters(ctx, userID, game.ID, rows); err != nil {
			slog.Error("bnetsync: replace", "user_id", userID, "err", err)
			return
		}
		if err := s.store.MarkWowSynced(ctx, userID, game.ID); err != nil {
			slog.Warn("bnetsync: mark synced", "user_id", userID, "err", err)
		}
	}
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func rawOrNil(r json.RawMessage) []byte {
	if len(r) == 0 {
		return nil
	}
	return r
}

// RefreshBnetGame refreshes a member's Diablo III or StarCraft II profile for
// the given game slug, on demand and per-user throttled. Safe to call on every
// page view. No-op for other slugs.
func (s *Syncer) RefreshBnetGame(ctx context.Context, userID, gameSlug string) {
	if s.bnet == nil || !s.bnet.Enabled() {
		return // connector not configured on this instance
	}
	scope := map[string]string{
		"diablo-iii":                "d3.profile",
		"starcraft-ii-battle-chest": "sc2.profile",
	}[gameSlug]
	if scope == "" {
		return // not a generic-profile game
	}

	tok, err := s.store.GetLinkedToken(ctx, userID, "battlenet")
	if err != nil {
		return
	}
	if tok.TokenExpiresAt == nil || tok.TokenExpiresAt.Before(time.Now()) {
		return
	}
	if !hasScope(tok.Scopes, scope) {
		return
	}

	game, err := s.store.GetGameBySlug(ctx, gameSlug)
	if err != nil {
		return
	}
	if synced, err := s.store.BnetSyncedAt(ctx, userID, game.ID); err == nil &&
		!synced.IsZero() && time.Since(synced) < throttle {
		return
	}

	access, err := s.cipher.OpenString(tok.AccessTokenEnc)
	if err != nil {
		slog.Warn("bnetsync: decrypt token (game)", "user_id", userID, "slug", gameSlug, "err", err)
		return
	}

	var data []byte
	switch gameSlug {
	case "diablo-iii":
		battleTag := ""
		if tok.DisplayName != nil {
			battleTag = *tok.DisplayName
		}
		if battleTag == "" {
			return // D3 endpoint is keyed by BattleTag; nothing to query without it
		}
		p, err := s.bnet.D3Profile(ctx, access, battleTag, s.region)
		if err != nil {
			slog.Warn("bnetsync: d3", "user_id", userID, "err", err)
			return
		}
		if p == nil {
			// No D3/SC2 profile for this member; mark synced anyway so we don't
			// re-hit Blizzard on every page view (throttled like a real sync).
			_ = s.store.MarkBnetSynced(ctx, userID, game.ID)
			return
		}
		data, err = json.Marshal(p)
		if err != nil {
			return
		}
	case "starcraft-ii-battle-chest":
		p, err := s.bnet.SC2Profile(ctx, access, tok.ProviderUserID, s.region)
		if err != nil {
			slog.Warn("bnetsync: sc2", "user_id", userID, "err", err)
			return
		}
		if p == nil {
			// No D3/SC2 profile for this member; mark synced anyway so we don't
			// re-hit Blizzard on every page view (throttled like a real sync).
			_ = s.store.MarkBnetSynced(ctx, userID, game.ID)
			return
		}
		data, err = json.Marshal(p)
		if err != nil {
			return
		}
	}

	if err := s.store.UpsertGameProfile(ctx, userID, game.ID, data); err != nil {
		slog.Error("bnetsync: upsert game profile", "user_id", userID, "err", err)
		return
	}
	if err := s.store.MarkBnetSynced(ctx, userID, game.ID); err != nil {
		slog.Warn("bnetsync: mark synced", "user_id", userID, "err", err)
	}
}
