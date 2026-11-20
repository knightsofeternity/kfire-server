package wowplayed

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

var guildNow = time.Date(2026, 11, 20, 22, 0, 0, 0, time.UTC)

func hours24(first string) string {
	h := []string{first}
	for i := 1; i < 24; i++ {
		h = append(h, "null")
	}
	return "[" + strings.Join(h, ",") + "]"
}

func TestParseGuildKeepsValidLinesOnly(t *testing.T) {
	raw := json.RawMessage(`{"game_slug":"world-of-warcraft-forever","characters":[],"guild":{
		"addon_version":"0.2.0","guild_name":"Knights of Eternity","realm_norm":"Chogall",
		"snapshot_at":"2026-11-20T21:04:00+00:00",
		"me":{"pseudo":"Oura","main":"Ouranos-Chogall"},
		"roster":[
			{"name":"Ouranos-Chogall","level":60,"class":"MAGE","rank":"Officier","rank_index":1,"last_seen_at":"2026-11-20T21:04:00+00:00"},
			{"name":"NoRealm","level":60},
			{"name":"Lvl-Chogall","level":999},
			{"name":"Ouranos-Chogall","level":1},
			{"name":"Future-Chogall","last_seen_at":"2027-01-01T00:00:00Z"},
			"junk",
			{"name":"Malkai-Chogall"}],
		"members":[
			{"name":"Ouranos-Chogall","pseudo":"Oura","main":"Ouranos-Chogall","role":"D","addon_version":"0.2.0","updated_at":"2026-11-20T20:00:00+00:00"},
			{"name":"Ouranos-Chogall","pseudo":"Old","role":"T","updated_at":"2026-11-19T20:00:00+00:00"},
			{"name":"Bad-Chogall","role":"X","updated_at":"2026-11-20T20:00:00+00:00"},
			{"name":"NoDate-Chogall","role":null}],
		"professions":[
			{"name":"Malkai-Chogall","profession":"Couture","level":75,"max":150,"recipes":["2996","4343","s7418","2996"],"updated_at":"2026-11-20T19:00:00+00:00"},
			{"name":"Malkai-Chogall","profession":"Cuisine","recipes":["abc"],"updated_at":"2026-11-20T19:00:00+00:00"}],
		"item_names":{"2996":"Rouleau d'étoffe de lin","s7418":"Sort","bad":"x","4343":5},
		"presence":[
			{"day":"2026-11-20","hours":` + hours24("3") + `},
			{"day":"2026-11-20","hours":` + hours24("5") + `},
			{"day":"2026-11-19","hours":[1,2]},
			{"day":"2026-11-18","hours":` + hours24("1001") + `},
			{"day":"20261117","hours":` + hours24("1") + `}]
	}}`)
	r, err := parseGuild(raw, guildNow)
	if err != nil || r == nil {
		t.Fatalf("parseGuild: %v", err)
	}
	if r.GuildKey != "knights of eternity" || r.GuildName != "Knights of Eternity" || r.RealmNorm != "Chogall" {
		t.Errorf("identity = %q %q %q", r.GuildKey, r.GuildName, r.RealmNorm)
	}
	if len(r.Roster) != 2 || r.Roster[0].Name != "Ouranos-Chogall" || *r.Roster[0].Level != 60 || r.Roster[1].Name != "Malkai-Chogall" {
		t.Errorf("roster = %+v, want Ouranos (first copy) and Malkai", r.Roster)
	}
	if len(r.Members) != 1 || *r.Members[0].Pseudo != "Oura" {
		t.Errorf("members = %+v, want only Ouranos' newest profile", r.Members)
	}
	if len(r.Professions) != 1 || fmt.Sprint(r.Professions[0].Recipes) != "[2996 4343 s7418]" {
		t.Errorf("professions = %+v, want Couture with deduplicated recipes", r.Professions)
	}
	if len(r.ItemNames) != 2 || r.ItemNames["s7418"] != "Sort" {
		t.Errorf("item names = %v", r.ItemNames)
	}
	if len(r.Presence) != 1 || *r.Presence[0].Hours[0] != 5 || r.Presence[0].Hours[1] != nil {
		t.Errorf("presence = %+v, want one merged day with 5 at 0h", r.Presence)
	}
}

func TestParseGuildAbsentOrInvalid(t *testing.T) {
	if r, err := parseGuild(json.RawMessage(`{"characters":[]}`), guildNow); r != nil || err != nil {
		t.Fatalf("no guild: %+v %v, want nil, nil", r, err)
	}
	many := make([]string, maxRoster+1)
	for i := range many {
		many[i] = `{"name":"A` + fmt.Sprint(i) + `-R"}`
	}
	for name, raw := range map[string]string{
		"not an object": `{"guild":"x"}`,
		"no name":       `{"guild":{"realm_norm":"R","snapshot_at":"2026-11-20T21:00:00Z"}}`,
		"no realm":      `{"guild":{"guild_name":"G","snapshot_at":"2026-11-20T21:00:00Z"}}`,
		"no snapshot":   `{"guild":{"guild_name":"G","realm_norm":"R"}}`,
		"future":        `{"guild":{"guild_name":"G","realm_norm":"R","snapshot_at":"2026-12-20T21:00:00Z"}}`,
		"roster limit":  `{"guild":{"guild_name":"G","realm_norm":"R","snapshot_at":"2026-11-20T21:00:00Z","roster":[` + strings.Join(many, ",") + `]}}`,
		"roster object": `{"guild":{"guild_name":"G","realm_norm":"R","snapshot_at":"2026-11-20T21:00:00Z","roster":{}}}`,
	} {
		if r, err := parseGuild(json.RawMessage(raw), guildNow); !errors.Is(err, errGuild) || r != nil {
			t.Errorf("%s: %+v %v, want errGuild", name, r, err)
		}
	}
}

// An invalid guild never fails the /played: Record only logs it. The /played
// check runs first and the guild is parsed independently of it.
func TestGuildDoesNotTouchPlayed(t *testing.T) {
	raw := json.RawMessage(`{"characters":[{"region":"eu","realm":"Cho'gall","realm_norm":"Chogall","name":"Ouranos",
		"played_seconds":10,"recorded_at":"2026-11-20T21:00:00Z"}],"guild":"broken"}`)
	if chars, err := accepted(raw, guildNow); err != nil || len(chars) != 1 {
		t.Fatalf("the /played must pass despite a broken guild: %v %v", chars, err)
	}
	// Other editions never look at the guild.
	(&Recorder{slug: "world-of-warcraft"}).recordGuild(t.Context(), "u", "g", raw)
}

func TestCharName(t *testing.T) {
	for s, want := range map[string]bool{
		"Ouranos-Chogall": true, "Ab-Azjol-Nerub": true, "A-Realm": false, "Name-": false,
		"Ouranos": false, "Ouranos-" + strings.Repeat("r", 65): false,
	} {
		if charName(s) != want {
			t.Errorf("charName(%q) = %v", s, !want)
		}
	}
}

func TestTypicalWeek(t *testing.T) {
	n := func(v int) *int { return &v }
	mon := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC) // a Monday
	var a, b, c store.WowGuildPresenceDay
	a.Day, b.Day, c.Day = mon, mon.AddDate(0, 0, 7), mon.AddDate(0, 0, 6) // Mon, Mon, Sun
	a.Hours[21], b.Hours[21] = n(4), n(7)
	b.Hours[20] = n(0)
	c.Hours[22] = n(9)
	week, best := TypicalWeek([]store.WowGuildPresenceDay{a, b, c}, 5)
	if week[0][21] == nil || *week[0][21] != 5.5 {
		t.Errorf("Monday 21h = %v, want 5.5", week[0][21])
	}
	if week[0][20] == nil || *week[0][20] != 0 || week[0][19] != nil {
		t.Errorf("a watched empty hour is 0, an unwatched one nil")
	}
	if len(best) != 2 || best[0] != (Slot{Weekday: 6, Hour: 22, Average: 9}) || best[1].Hour != 21 {
		t.Errorf("best = %+v", best)
	}
	if got := PresenceSince(time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)); got != time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) {
		t.Errorf("since = %v", got)
	}
}
