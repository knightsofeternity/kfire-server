package epicsync

import (
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/epic"
)

func TestIsGame(t *testing.T) {
	cases := []struct {
		cats []string
		want bool
	}{
		{[]string{"games", "applications"}, true},           // Fortnite
		{[]string{"public", "games", "applications"}, true}, // Rocket League
		{[]string{"hidden"}, false},                         // Contenu de LEGO Fortnite
		{[]string{"games", "hidden"}, false},
		{[]string{"applications"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := IsGame(c.cats); got != c.want {
			t.Errorf("IsGame(%v) = %v", c.cats, got)
		}
	}
}

// One Fortnite session shows up three times in Epic's playtime (the game and
// two add-ons, 221 s each on 2026-10-04): only the game counts.
func TestSecondsByGame(t *testing.T) {
	games := map[string]string{"Fortnite": "g-fn", "Sugar": "g-rl", "bobcat": "g-sw"}
	play := []epic.Playtime{
		{ArtifactID: "Fortnite", TotalTime: 412468},
		{ArtifactID: "94bc5ec13f8f438c97fdbef3e9019e27", TotalTime: 221},
		{ArtifactID: "aa31f9e94e844b299ca757d1d0b97a09", TotalTime: 221},
		{ArtifactID: "bobcat", TotalTime: 97},
	}
	got := secondsByGame(games, play)
	if len(got) != 3 || got["g-fn"] != 412468 || got["g-sw"] != 97 || got["g-rl"] != 0 {
		t.Fatalf("got %v: every owned game gets a row, add-ons never count", got)
	}
}
