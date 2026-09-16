package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLivePayloadValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"en cours", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143,"overtime":false,"goals":1,"assists":0,"saves":2,"shots":3,"score":310,"demos":0}`, true},
		{"prolongation", `{"game_slug":"rocket-league","team_blue_score":3,"team_orange_score":3,"seconds_remaining":47,"overtime":true,"goals":1,"assists":1,"saves":0,"shots":2,"score":280,"demos":1}`, true},
		{"fin de match", `{"game_slug":"rocket-league","ended":true}`, true},
		{"slug manquant", `{"team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"score negatif", `{"game_slug":"rocket-league","team_blue_score":-1,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"chrono negatif", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":-5}`, false},
		{"chrono aberrant", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":99999}`, false},
		{"stat negative", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143,"saves":-2}`, false},
		{"score de match aberrant", `{"game_slug":"rocket-league","team_blue_score":999,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"slug avec du html", `{"game_slug":"<script>alert(1)</script>","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"slug a rallonge", fmt.Sprintf(`{"game_slug":%q,"team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, strings.Repeat("a", 200)), false},
		{"slug majuscule", `{"game_slug":"Rocket-League","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"stat aberrante", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143,"score":2147483647}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p livePayload
			err := json.Unmarshal([]byte(tc.body), &p)
			got := err == nil && p.valid()
			if got != tc.ok {
				t.Errorf("valid() = %v, want %v", got, tc.ok)
			}
		})
	}
}

func TestLiveExpire(t *testing.T) {
	now := time.Now()
	e := liveEntry{updatedAt: now.Add(-liveTTL - time.Second)}
	if !e.expired(now) {
		t.Errorf("an entry older than the TTL must be expired")
	}
	fresh := liveEntry{updatedAt: now.Add(-time.Second)}
	if fresh.expired(now) {
		t.Errorf("an entry from a second ago must not be expired")
	}
}

func TestLiveSharedState(t *testing.T) {
	// A hub with no dependencies is enough: setLive and LiveMatch only touch
	// the in-memory map. That is what makes this state testable while the
	// rest of the hub is not.
	h := NewHub(nil, nil, "", nil)

	p := livePayload{
		GameSlug: "rocket-league", TeamBlueScore: 2, TeamOrangeScore: 1,
		SecondsRemaining: 143, Goals: 1, Saves: 2, Shots: 3, Score: 310,
	}
	h.setLive("u1", p)

	got := h.LiveMatch("u1")
	if got == nil {
		t.Fatal("LiveMatch returns nil right after setLive")
	}
	if got["team_blue_score"] != 2 || got["seconds_remaining"] != 143 {
		t.Errorf("state read back incorrectly: %v", got)
	}
	if _, named := got["user_id"]; named {
		t.Error("the broadcast state must not carry identity, the hub adds it around")
	}

	// A member with no match in progress has no state.
	if h.LiveMatch("unknown") != nil {
		t.Error("LiveMatch returns a state for a member who is not playing")
	}

	// The end of a match clears it.
	h.setLive("u1", livePayload{GameSlug: "rocket-league", Ended: true})
	if h.LiveMatch("u1") != nil {
		t.Error("the state survives the end of the match")
	}
}

// This test only has value run with -race: it makes reads and writes on the
// shared map cross paths, which no other test in the package does. Without
// it, `go test -race ./internal/ws/` proves nothing about h.live.
func TestLiveConcurrentAccess(t *testing.T) {
	h := NewHub(nil, nil, "", nil)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		member := fmt.Sprintf("u%d", i%3)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.setLive(member, livePayload{GameSlug: "rocket-league", TeamBlueScore: j % 5})
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = h.LiveMatch(member)
			}
		}()
	}
	wg.Wait()
}

func TestLiveVisibilityCutsTheStream(t *testing.T) {
	h := NewHub(nil, nil, "", nil)
	h.setLiveVisible("u1", true, "online")
	if !h.liveAllowed("u1") {
		t.Fatal("a visible member must be able to broadcast")
	}
	h.setLive("u1", livePayload{GameSlug: "rocket-league", TeamBlueScore: 1})

	// They go hidden mid-match.
	h.SetVisibility("u1", true, "invisible")
	if h.liveAllowed("u1") {
		t.Error("an invisible member must no longer broadcast")
	}
	if h.LiveMatch("u1") != nil {
		t.Error("their live match should have disappeared immediately")
	}

	// A member who hides their activity without declaring themselves
	// invisible, too.
	h.setLiveVisible("u2", false, "online")
	if h.liveAllowed("u2") {
		t.Error("hidden activity must be enough to cut the stream")
	}
}

// The visibility toggle arrives over an HTTP goroutine while the
// connection's read loop consults the permission. This is exactly the race
// that made the first version of this fix unacceptable.
func TestLiveVisibilityConcurrentAccess(t *testing.T) {
	h := NewHub(nil, nil, "", nil)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				h.SetVisibility("u1", j%2 == 0, "online")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				if h.liveAllowed("u1") {
					h.setLive("u1", livePayload{GameSlug: "rocket-league"})
				}
			}
		}()
	}
	wg.Wait()
}
