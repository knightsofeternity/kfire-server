package bnetsync

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/knightsofeternity/kfire-server/internal/connectors/battlenet"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// WowPlugin exposes World of Warcraft (retail + classic) as a game plugin,
// wrapping the bnet syncer + store reads.
type WowPlugin struct {
	st     *store.Store
	syncer *Syncer
	conn   *battlenet.Connector
}

// NewWowPlugin builds the WoW plugin.
func NewWowPlugin(st *store.Store, s *Syncer, conn *battlenet.Connector) *WowPlugin {
	return &WowPlugin{st: st, syncer: s, conn: conn}
}

func (p *WowPlugin) ID() string        { return "wow" }
func (p *WowPlugin) Name() string      { return "World of Warcraft" }
func (p *WowPlugin) Connector() string { return "battlenet" }
func (p *WowPlugin) Available() bool   { return p.conn.Enabled() }
func (p *WowPlugin) Slugs() []string {
	return []string{"world-of-warcraft", "world-of-warcraft-classic"}
}

func (p *WowPlugin) Refresh(ctx context.Context, userID, gameSlug string) {
	if !slices.Contains(p.Slugs(), gameSlug) {
		return
	}
	p.syncer.RefreshWoW(ctx, userID, gameSlug)
}

// GameDetail returns the aggregate wow_characters block for the game page.
func (p *WowPlugin) GameDetail(ctx context.Context, _ string, g store.Game) (map[string]any, error) {
	chars, synced, err := p.st.WowCharactersByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	cards := make([]map[string]any, len(chars))
	for i, ch := range chars {
		m := map[string]any{
			"user_id": ch.UserID, "name": ch.Name, "realm": ch.RealmName,
			"class": ch.Class, "race": ch.Race, "faction": ch.Faction,
			"level": ch.Level, "item_level": ch.ItemLevel,
			"achievement_points": ch.AchievementPoints,
		}
		if ch.MythicRating != nil {
			m["mythic_rating"] = *ch.MythicRating
		}
		if len(ch.RaidSummary) > 0 {
			m["raid_summary"] = json.RawMessage(ch.RaidSummary)
		}
		cards[i] = m
	}
	return map[string]any{"wow_characters": cards, "wow_synced_at": synced}, nil
}

// UserGameDetail returns one member's wow_characters block.
func (p *WowPlugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	chars, err := p.st.WowCharactersForUserGame(ctx, userID, g.ID)
	// No characters: omit the block entirely (the per-user page only shows it when populated), unlike the aggregate GameDetail which always returns an empty list + sync timestamp.
	if err != nil || len(chars) == 0 {
		return nil, err
	}
	cards := make([]map[string]any, len(chars))
	for i, ch := range chars {
		m := map[string]any{
			"name": ch.Name, "realm": ch.RealmName, "realm_slug": ch.RealmSlug,
			"class": ch.Class, "race": ch.Race, "faction": ch.Faction,
			"level": ch.Level, "item_level": ch.ItemLevel,
			"achievement_points": ch.AchievementPoints,
		}
		if ch.MythicRating != nil {
			m["mythic_rating"] = *ch.MythicRating
		}
		if len(ch.RaidSummary) > 0 {
			m["raid_summary"] = json.RawMessage(ch.RaidSummary)
		}
		cards[i] = m
	}
	return map[string]any{"wow_characters": cards}, nil
}

// BnetProfilePlugin exposes a single Battle.net profile game (Diablo III or
// StarCraft II) whose stats are an opaque JSON blob.
type BnetProfilePlugin struct {
	st             *store.Store
	syncer         *Syncer
	conn           *battlenet.Connector
	id, name, slug string
}

// NewBnetProfilePlugin builds a profile-blob plugin for one slug.
func NewBnetProfilePlugin(st *store.Store, s *Syncer, conn *battlenet.Connector, id, name, slug string) *BnetProfilePlugin {
	return &BnetProfilePlugin{st: st, syncer: s, conn: conn, id: id, name: name, slug: slug}
}

func (p *BnetProfilePlugin) ID() string        { return p.id }
func (p *BnetProfilePlugin) Name() string      { return p.name }
func (p *BnetProfilePlugin) Connector() string { return "battlenet" }
func (p *BnetProfilePlugin) Available() bool   { return p.conn.Enabled() }
func (p *BnetProfilePlugin) Slugs() []string   { return []string{p.slug} }

func (p *BnetProfilePlugin) Refresh(ctx context.Context, userID, gameSlug string) {
	if gameSlug != p.slug {
		return
	}
	p.syncer.RefreshBnetGame(ctx, userID, gameSlug)
}

// GameDetail returns the aggregate bnet_profiles block for the game page.
func (p *BnetProfilePlugin) GameDetail(ctx context.Context, _ string, g store.Game) (map[string]any, error) {
	profs, synced, err := p.st.GameProfilesByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(profs))
	for i, pr := range profs {
		out[i] = map[string]any{
			"user_id":  pr.UserID,
			"username": pr.Username,
			"data":     json.RawMessage(pr.Data),
		}
	}
	return map[string]any{"bnet_profiles": out, "bnet_synced_at": synced}, nil
}

// UserGameDetail returns one member's bnet_profile blob.
func (p *BnetProfilePlugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	data, err := p.st.GameProfileForUserGame(ctx, userID, g.ID)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	return map[string]any{"bnet_profile": json.RawMessage(data)}, nil
}
