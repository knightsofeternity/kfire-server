package wowplayed

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
)

func TestAcceptedKeepsValidCharactersOnly(t *testing.T) {
	now := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	raw := json.RawMessage(`{"game_slug":"world-of-warcraft","characters":[
		{"region":"eu","realm":"Cho'gall","realm_norm":"Cho'gall","name":"Ouranos","played_seconds":17587583,"level":80,"class":"PRIEST","recorded_at":"2026-10-01T17:58:00Z"},
		{"region":"mars","realm":"X","realm_norm":"X","name":"Bad","played_seconds":1,"recorded_at":"2026-10-01T17:58:00Z"},
		{"region":"eu","realm":"X","realm_norm":"X","name":"Future","played_seconds":1,"recorded_at":"2026-10-02T17:58:00Z"},
		{"region":"eu","realm":"X","realm_norm":"X","name":"Negative","played_seconds":-5,"recorded_at":"2026-10-01T17:58:00Z"},
		{"region":"unknown","realm":"Laughing Skull","realm_norm":"LaughingSkull","name":"Trayn","played_seconds":10152,"recorded_at":"2026-10-01T17:00:00Z"}]}`)
	got, err := accepted(raw, now)
	if err != nil || len(got) != 2 || got[0].Name != "Ouranos" || got[1].Region != "unknown" {
		t.Fatalf("got %+v err %v: the two valid characters must pass, the others drop", got, err)
	}
}

func TestAcceptedRefusesEmptyOrJunk(t *testing.T) {
	now := time.Now()
	for _, raw := range []string{
		`{`, `{"characters":[]}`,
		`{"characters":[{"region":"eu","realm":"R","realm_norm":"R","name":"x","played_seconds":1,"recorded_at":"2026-10-01T00:00:00Z"}]}`,
	} {
		if _, err := accepted(json.RawMessage(raw), now); !errors.Is(err, matchrecord.ErrInvalidPayload) {
			t.Errorf("%s: %v, want ErrInvalidPayload", raw, err)
		}
	}
}

func TestOneRecorderPerEdition(t *testing.T) {
	reg := matchrecord.NewRegistry(Recorders(nil)...)
	_ = reg
	seen := map[string]bool{}
	for _, r := range Recorders(nil) {
		seen[r.Slug()] = true
	}
	for _, s := range []string{"world-of-warcraft", "world-of-warcraft-classic", "world-of-warcraft-forever", "wow-ascension"} {
		if !seen[s] {
			t.Errorf("no recorder for %s", s)
		}
	}
}
