package riotsync

import (
	"encoding/json"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
	"github.com/knightsofeternity/kfire-server/internal/gameplugin"
)

// LolPlugin must satisfy the registry's interface.
var _ gameplugin.Plugin = (*LolPlugin)(nil)

func TestLolPluginIdentity(t *testing.T) {
	p := NewLolPlugin(nil, nil, riot.New("", "", ""))
	if p.ID() != "lol" {
		t.Errorf("ID() = %q, want lol", p.ID())
	}
	if p.Connector() != "riot" {
		t.Errorf("Connector() = %q, want riot", p.Connector())
	}
	if got := p.Slugs(); len(got) != 1 || got[0] != "league-of-legends" {
		t.Errorf("Slugs() = %v", got)
	}
	if p.Available() {
		t.Error("Available() must be false without credentials")
	}
}

func TestLolPluginAvailableWithCredentials(t *testing.T) {
	p := NewLolPlugin(nil, nil, riot.New("id", "secret", "RGAPI-x"))
	if !p.Available() {
		t.Error("Available() must be true once the connector is configured")
	}
}

func TestSortByScoreOrdersRankedFirstThenByUsername(t *testing.T) {
	cards := []map[string]any{
		{"username": "zoe", "solo_score": float64(-1)},
		{"username": "amy", "solo_score": float64(-1)},
		{"username": "bob", "solo_score": float64(300000)},
		{"username": "cal", "solo_score": float64(920000)},
	}
	sortByScore(cards)

	want := []string{"cal", "bob", "amy", "zoe"}
	for i, w := range want {
		if cards[i]["username"] != w {
			t.Fatalf("position %d = %v, want %s (order: %v)", i, cards[i]["username"], w, cards)
		}
	}
}

func TestScoreOfReadsTheBlob(t *testing.T) {
	blob, _ := json.Marshal(map[string]any{"solo_score": 12345})
	if got := scoreOf(blob); got != 12345 {
		t.Errorf("scoreOf = %d, want 12345", got)
	}
	if got := scoreOf([]byte(`{}`)); got != -1 {
		t.Errorf("scoreOf on a blob without the field = %d, want -1", got)
	}
	if got := scoreOf([]byte(`not json`)); got != -1 {
		t.Errorf("scoreOf on invalid JSON = %d, want -1", got)
	}
	if got := scoreOf(nil); got != -1 {
		t.Errorf("scoreOf(nil) = %d, want -1", got)
	}
}

func TestScoreOfAcceptsAZeroScore(t *testing.T) {
	blob, _ := json.Marshal(map[string]any{"solo_score": 0})
	if got := scoreOf(blob); got != 0 {
		t.Errorf("scoreOf = %d, want 0; iron IV at 0 LP is a real rank and must not "+
			"be confused with the unranked sentinel", got)
	}
}

// TestLiveIsGatedOnTheActivityToggle documents the privacy rule the leaderboard
// must follow: a member's standing is always shown, but what they are playing
// right now is only shown when they allow it, or to themselves.
func TestLiveIsGatedOnTheActivityToggle(t *testing.T) {
	cases := []struct {
		name            string
		activityVisible bool
		viewerID        string
		wantLive        bool
	}{
		{"visible member, any viewer", true, "someone-else", true},
		{"hidden member, other viewer", false, "someone-else", false},
		{"hidden member, looking at themselves", false, "u1", true},
		{"hidden member, anonymous viewer", false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := showLive(tc.activityVisible, "u1", tc.viewerID)
			if got != tc.wantLive {
				t.Errorf("live shown = %v, want %v", got, tc.wantLive)
			}
		})
	}
}
