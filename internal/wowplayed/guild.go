package wowplayed

// The KFireGuilde addon (lot 5) rides the Forever /played: the match_result
// may carry an optional `guild` object. Contract:
// kfire-client wow-addon/docs/kfire-guilde-liaison.md. A bad line is dropped,
// a bad guild is ignored, and neither ever fails the /played.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// GuildSlug is the only edition the KFireGuilde addon is installed in.
const GuildSlug = "world-of-warcraft-forever"

const (
	maxRoster      = 1000
	maxMembers     = 1000
	maxProfessions = 3000
	maxRecipes     = 1000
	maxItemNames   = 20000
	maxPresence    = 70
	maxPresenceN   = 1000
)

var (
	itemIDRe = regexp.MustCompile(`^s?[0-9]{1,10}$`)
	roles    = map[string]bool{"T": true, "H": true, "D": true}
	errGuild = errors.New("invalid guild")
)

type guildPayload struct {
	Guild *guildJSON `json:"guild"`
}

type guildJSON struct {
	AddonVersion *string           `json:"addon_version"`
	GuildName    string            `json:"guild_name"`
	RealmNorm    string            `json:"realm_norm"`
	SnapshotAt   time.Time         `json:"snapshot_at"`
	Roster       []json.RawMessage `json:"roster"`
	Members      []json.RawMessage `json:"members"`
	Professions  []json.RawMessage `json:"professions"`
	ItemNames    map[string]any    `json:"item_names"`
	Presence     []json.RawMessage `json:"presence"`
}

type rosterJSON struct {
	Name       string     `json:"name"`
	Level      *int       `json:"level"`
	Class      *string    `json:"class"`
	Rank       *string    `json:"rank"`
	RankIndex  *int       `json:"rank_index"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

type memberJSON struct {
	Name         string    `json:"name"`
	Pseudo       *string   `json:"pseudo"`
	Main         *string   `json:"main"`
	Role         *string   `json:"role"`
	AddonVersion *string   `json:"addon_version"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type professionJSON struct {
	Name       string    `json:"name"`
	Profession string    `json:"profession"`
	Level      *int      `json:"level"`
	Max        *int      `json:"max"`
	Recipes    []string  `json:"recipes"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type presenceJSON struct {
	Day   string `json:"day"`
	Hours []*int `json:"hours"`
}

// charName reports whether s is "Name-Realm": 2 to 24 characters of name, 1
// to 64 of realm. A WoW name never holds a hyphen; the realm may.
func charName(s string) bool {
	name, realm, ok := strings.Cut(s, "-")
	return ok && textOK(name, 2, 24) && textOK(realm, 1, 64)
}

// optText reports whether an optional text is absent or within bounds.
func optText(s *string, min, max int) bool {
	return s == nil || textOK(*s, min, max)
}

func optInt(n *int, min, max int) bool {
	return n == nil || (*n >= min && *n <= max)
}

// timeOK refuses a zero date or one from the future.
func timeOK(t time.Time, now time.Time) bool {
	return !t.IsZero() && !t.After(now.Add(clockSkew))
}

// parseGuild validates the guild object of a payload. It returns nil, nil when
// there is none; errGuild when the object as a whole is unusable. Lines that
// fail their own checks are dropped silently.
func parseGuild(raw json.RawMessage, now time.Time) (*store.WowGuildReport, error) {
	var p guildPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("%w: unreadable json: %v", errGuild, err)
	}
	g := p.Guild
	if g == nil {
		return nil, nil
	}
	if !textOK(g.GuildName, 1, 64) || !textOK(g.RealmNorm, 1, 64) || !timeOK(g.SnapshotAt, now) {
		return nil, fmt.Errorf("%w: name, realm or snapshot_at", errGuild)
	}
	if !optText(g.AddonVersion, 1, 32) {
		return nil, fmt.Errorf("%w: addon_version", errGuild)
	}
	if len(g.Roster) > maxRoster || len(g.Members) > maxMembers || len(g.Professions) > maxProfessions ||
		len(g.ItemNames) > maxItemNames || len(g.Presence) > maxPresence {
		return nil, fmt.Errorf("%w: over a limit (roster %d, members %d, professions %d, item_names %d, presence %d)",
			errGuild, len(g.Roster), len(g.Members), len(g.Professions), len(g.ItemNames), len(g.Presence))
	}
	r := &store.WowGuildReport{
		GuildKey:   strings.ToLower(g.GuildName),
		GuildName:  g.GuildName,
		RealmNorm:  g.RealmNorm,
		SnapshotAt: g.SnapshotAt.UTC(),
		ItemNames:  map[string]string{},
	}

	seen := map[string]bool{}
	for _, line := range g.Roster {
		var m rosterJSON
		if json.Unmarshal(line, &m) != nil || !charName(m.Name) || seen[m.Name] {
			continue
		}
		if !optInt(m.Level, 1, 200) || !optText(m.Class, 1, 32) || !optText(m.Rank, 1, 64) || !optInt(m.RankIndex, 0, 255) {
			continue
		}
		if m.LastSeenAt != nil {
			if !timeOK(*m.LastSeenAt, now) {
				continue
			}
			t := m.LastSeenAt.UTC()
			m.LastSeenAt = &t
		}
		seen[m.Name] = true
		r.Roster = append(r.Roster, store.WowGuildRosterRow{
			Name: m.Name, Level: m.Level, Class: m.Class, Rank: m.Rank, RankIndex: m.RankIndex, LastSeenAt: m.LastSeenAt,
		})
	}

	newest := map[string]int{} // name -> index in r.Members, the newest wins
	for _, line := range g.Members {
		var m memberJSON
		if json.Unmarshal(line, &m) != nil || !charName(m.Name) || !timeOK(m.UpdatedAt, now) {
			continue
		}
		if !optText(m.Pseudo, 1, 32) || (m.Main != nil && !charName(*m.Main)) || !optText(m.AddonVersion, 1, 32) {
			continue
		}
		if m.Role != nil && !roles[*m.Role] {
			continue
		}
		row := store.WowGuildMember{Name: m.Name, Pseudo: m.Pseudo, Main: m.Main, Role: m.Role,
			AddonVersion: m.AddonVersion, UpdatedAt: m.UpdatedAt.UTC()}
		if i, ok := newest[m.Name]; ok {
			if row.UpdatedAt.After(r.Members[i].UpdatedAt) {
				r.Members[i] = row
			}
			continue
		}
		newest[m.Name] = len(r.Members)
		r.Members = append(r.Members, row)
	}

	newest = map[string]int{}
	for _, line := range g.Professions {
		var p professionJSON
		if json.Unmarshal(line, &p) != nil || !charName(p.Name) || !textOK(p.Profession, 1, 64) || !timeOK(p.UpdatedAt, now) {
			continue
		}
		if !optInt(p.Level, 0, 1000) || !optInt(p.Max, 0, 1000) || len(p.Recipes) > maxRecipes {
			continue
		}
		recipes := make([]string, 0, len(p.Recipes))
		dup := map[string]bool{}
		bad := false
		for _, id := range p.Recipes {
			if !itemIDRe.MatchString(id) {
				bad = true
				break
			}
			if !dup[id] {
				dup[id] = true
				recipes = append(recipes, id)
			}
		}
		if bad {
			continue
		}
		row := store.WowGuildProfession{Name: p.Name, Profession: p.Profession, Level: p.Level, Max: p.Max,
			Recipes: recipes, UpdatedAt: p.UpdatedAt.UTC()}
		key := p.Name + "/" + p.Profession
		if i, ok := newest[key]; ok {
			if row.UpdatedAt.After(r.Professions[i].UpdatedAt) {
				r.Professions[i] = row
			}
			continue
		}
		newest[key] = len(r.Professions)
		r.Professions = append(r.Professions, row)
	}

	for id, v := range g.ItemNames {
		name, ok := v.(string)
		if ok && itemIDRe.MatchString(id) && textOK(name, 1, 128) {
			r.ItemNames[id] = name
		}
	}

	days := map[string]int{}
	tomorrow := now.UTC().AddDate(0, 0, 1)
	for _, line := range g.Presence {
		var d presenceJSON
		if json.Unmarshal(line, &d) != nil || len(d.Hours) != 24 {
			continue
		}
		day, err := time.Parse("2006-01-02", d.Day)
		if err != nil || day.After(tomorrow) {
			continue
		}
		var row store.WowGuildPresenceDay
		row.Day = day
		ok := true
		for i, h := range d.Hours {
			if h != nil && (*h < 0 || *h > maxPresenceN) {
				ok = false
				break
			}
			row.Hours[i] = h
		}
		if !ok {
			continue
		}
		if i, seen := days[d.Day]; seen {
			// Two lines for one day merge like two reports: the larger count.
			for h := range row.Hours {
				r.Presence[i].Hours[h] = maxHour(r.Presence[i].Hours[h], row.Hours[h])
			}
			continue
		}
		days[d.Day] = len(r.Presence)
		r.Presence = append(r.Presence, row)
	}
	return r, nil
}

func maxHour(a, b *int) *int {
	switch {
	case a == nil:
		return b
	case b == nil || *a >= *b:
		return a
	}
	return b
}

// recordGuild writes the guild object of a Forever payload, if any. Every
// failure is logged and swallowed: the /played has its own fate.
func (r *Recorder) recordGuild(ctx context.Context, userID, gameID string, raw json.RawMessage) {
	if r.slug != GuildSlug || r.st == nil {
		return
	}
	rep, err := parseGuild(raw, time.Now())
	if err != nil {
		slog.Warn("wowplayed: guild ignored", "user_id", userID, "err", err)
		return
	}
	if rep == nil {
		return
	}
	if err := r.st.RecordWowGuild(ctx, userID, gameID, *rep); err != nil {
		slog.Error("wowplayed: guild not recorded", "user_id", userID, "guild", rep.GuildName,
			"realm", rep.RealmNorm, "err", err)
	}
}
