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
	// Throttle once per game: skip if this game's data was refreshed recently.
	if _, newest, err := s.store.WowCharactersByGame(ctx, game.ID); err == nil &&
		!newest.IsZero() && time.Since(newest) < throttle {
		return
	}

	access, err := s.cipher.OpenString(tok.AccessTokenEnc)
	if err != nil {
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
		anyOK = true
		for i := range chars {
			if err := s.bnet.EnrichWowCharacter(ctx, access, ns.namespace, &chars[i]); err != nil {
				slog.Warn("bnetsync: enrich", "char", chars[i].Name, "err", err)
				continue
			}
			c := chars[i]
			rows = append(rows, store.WowCharacterRow{
				Region: s.region, RealmSlug: c.RealmSlug, Name: c.Name, RealmName: c.RealmName,
				Faction: strPtr(c.Faction), Race: strPtr(c.Race), Class: strPtr(c.Class),
				Level: c.Level, ItemLevel: c.ItemLevel, MythicRating: c.MythicRating,
				RaidSummary: rawOrNil(c.RaidSummary),
			})
		}
	}
	if anyOK {
		if err := s.store.ReplaceWowCharacters(ctx, userID, game.ID, rows); err != nil {
			slog.Error("bnetsync: replace", "user_id", userID, "err", err)
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
