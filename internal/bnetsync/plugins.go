package bnetsync

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

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
	// Forever and Ascension have no Battle.net profile, but their /played
	// comes from the KFire addon and shows on the same character cards.
	return []string{"world-of-warcraft", "world-of-warcraft-classic", "world-of-warcraft-forever", "wow-ascension"}
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
			"user_id": ch.UserID, "username": ch.Username,
			"name": ch.Name, "realm": ch.RealmName,
			"class": ch.Class, "race": ch.Race, "faction": ch.Faction,
			"level": ch.Level, "item_level": ch.ItemLevel,
			"achievement_points": ch.AchievementPoints,
		}
		if ch.AvatarURL != nil {
			m["avatar_url"] = *ch.AvatarURL
		}
		if ch.Version != nil {
			m["version"] = *ch.Version
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

// realmKey compares a Battle.net realm (slug "chogall", name "Cho'gall")
// with the addon's ("Cho'gall"): lowercase, without spaces, quotes or dashes.
func realmKey(realm, name string) string {
	r := strings.NewReplacer(" ", "", "'", "", "-", "", "’", "").Replace(strings.ToLower(realm))
	return r + "/" + strings.ToLower(name)
}

// UserGameDetail returns one member's wow_characters block: the Battle.net
// characters, each with its /played when the KFire addon recorded it, and the
// characters only the addon knows (Forever, Ascension, or a character
// Blizzard no longer indexes).
func (p *WowPlugin) UserGameDetail(ctx context.Context, userID string, g store.Game) (map[string]any, error) {
	chars, err := p.st.WowCharactersForUserGame(ctx, userID, g.ID)
	if err != nil {
		return nil, err
	}
	played, err := p.st.WowPlayedForUserGame(ctx, userID, g.ID)
	if err != nil {
		return nil, err
	}
	// No characters at all: omit the block entirely (the per-user page only shows it when populated), unlike the aggregate GameDetail which always returns an empty list + sync timestamp.
	if len(chars) == 0 && len(played) == 0 {
		return nil, nil
	}
	playedBy := make(map[string]store.WowPlayed, len(played))
	for _, c := range played {
		playedBy[realmKey(c.Realm, c.Name)] = c
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
		if ch.Version != nil {
			m["version"] = *ch.Version
		}
		// Whether Blizzard has ever handed us this character's achievements.
		// False means its profile answers 404, which happens to characters left
		// unplayed; the page says so instead of showing an empty list.
		m["has_achievements"] = ch.HasAchievements
		for _, key := range []string{realmKey(ch.RealmSlug, ch.Name), realmKey(ch.RealmName, ch.Name)} {
			if c, ok := playedBy[key]; ok {
				m["played_seconds"] = c.PlayedSeconds
				delete(playedBy, key)
				break
			}
		}
		cards[i] = m
	}
	// Characters only the addon knows, in the order of their /played.
	for _, c := range played {
		if _, still := playedBy[realmKey(c.Realm, c.Name)]; !still {
			continue
		}
		m := map[string]any{
			"name": c.Name, "realm": c.Realm, "played_seconds": c.PlayedSeconds,
			"item_level": 0, "has_achievements": false, "addon_only": true,
		}
		if c.Class != nil {
			m["class"] = wowClassName(*c.Class)
		}
		if c.Level != nil {
			m["level"] = *c.Level
		}
		if g.Slug == "world-of-warcraft" {
			m["version"] = "retail"
		}
		cards = append(cards, m)
	}

	out := map[string]any{"wow_characters": cards}
	if len(chars) == 0 {
		return out, nil
	}
	out["wow_synced_at"] = chars[0].LastSyncedAt

	// A Battle.net token lasts 24 hours and Blizzard issues no refresh token,
	// so a member's characters freeze the day after they link. Saying nothing
	// would leave them reading months-old data as if it were current.
	if tok, err := p.st.GetLinkedToken(ctx, userID, "battlenet"); err == nil {
		out["wow_link_expired"] = tok.TokenExpiresAt == nil || tok.TokenExpiresAt.Before(time.Now())
	}
	return out, nil
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

// wowClassNames lists every class name the addon may report, Blizzard's and
// Project Ascension's own (Conquest of Azeroth). The game's token is the name
// in capitals without spaces ("SONOFARUGAL"), which is how they are matched.
var wowClassNames = func() map[string]string {
	names := []string{
		"Warrior", "Paladin", "Hunter", "Rogue", "Priest", "Death Knight", "Shaman",
		"Mage", "Warlock", "Monk", "Druid", "Demon Hunter", "Evoker",
		"Barbarian", "Bloodmage", "Chronomancer", "Cultist", "Felsworn", "Guardian",
		"Knight of Xoroth", "Necromancer", "Primalist", "Prophet", "Pyromancer",
		"Ranger", "Reaper", "Runemaster", "Son of Arugal", "Spiritmage",
		"Starcaller", "Stormbringer", "Sun Cleric", "Templar", "Tinker",
		"Venomancer", "Witch Doctor", "Witch Hunter",
	}
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[strings.ToUpper(strings.ReplaceAll(n, " ", ""))] = n
	}
	return m
}()

// wowClassName turns the addon's class token ("DEATHKNIGHT") into the name
// Battle.net cards use ("Death Knight"), which the page colours and iconises.
// A class it does not know still reads as a word ("Lightbringer"), not a token.
func wowClassName(token string) string {
	up := strings.ToUpper(token)
	if n, ok := wowClassNames[up]; ok {
		return n
	}
	if up == "" {
		return ""
	}
	return up[:1] + strings.ToLower(up[1:])
}
