package ws

import (
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

func TestPlatformOf(t *testing.T) {
	for src, want := range map[string]string{"psn_api": "playstation", "xbox_api": "xbox", "nintendo_api": "nintendo", "client": "", "": "", "steam_api": ""} {
		if got := PlatformOf(src); got != want {
			t.Errorf("PlatformOf(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestGamesJSONKeepsEveryGameAndItsConsole(t *testing.T) {
	now := time.Now()
	open := []store.Session{
		{Source: "nintendo_api", StartedAt: now, Game: store.Game{Name: "Mario Kart World"}},
		{Source: "client", StartedAt: now.Add(-time.Hour), Game: store.Game{Name: "World of Warcraft"}},
	}
	got := GamesJSON(open, func(g store.Game) map[string]any { return map[string]any{"name": g.Name} })
	if len(got) != 2 {
		t.Fatalf("got %d games, want 2", len(got))
	}
	if got[0]["platform"] != "nintendo" || got[0]["game"].(map[string]any)["name"] != "Mario Kart World" {
		t.Fatalf("first = %v", got[0])
	}
	if _, ok := got[1]["platform"]; ok {
		t.Fatal("a desktop client session has no console platform")
	}
}
