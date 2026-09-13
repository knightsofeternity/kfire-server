package riotsync

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
)

func TestBuildProfilePutsEverythingTheSPANeeds(t *testing.T) {
	blob := buildProfile(
		"Cäps#EUW", "euw1",
		[]riot.RankEntry{{Queue: "RANKED_SOLO_5x5", Tier: "GOLD", Division: "II", LP: 47, Wins: 120, Losses: 108}},
		[]riot.ChampionMastery{{ChampionID: 202, Name: "Jhin", Level: 4, Points: 12600}},
		[]riot.MatchResult{{MatchID: "EUW1_1", Win: true, Champion: "Jhin", PlayedAt: time.Unix(0, 0).UTC()}},
	)

	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("buildProfile produced invalid JSON: %v", err)
	}
	if got["riot_id"] != "Cäps#EUW" {
		t.Errorf("riot_id = %v", got["riot_id"])
	}
	if got["platform"] != "euw1" {
		t.Errorf("platform = %v", got["platform"])
	}
	want := float64(SoloScore([]riot.RankEntry{
		{Queue: "RANKED_SOLO_5x5", Tier: "GOLD", Division: "II", LP: 47},
	}))
	if got["solo_score"].(float64) != want {
		t.Errorf("solo_score = %v, want %v", got["solo_score"], want)
	}
	for _, key := range []string{"ranks", "top_champions", "recent"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
}

func TestBuildProfileOnAnUnrankedAccountEmitsEmptyListsNotNull(t *testing.T) {
	blob := buildProfile("New#EUW", "euw1", nil, nil, nil)

	var got struct {
		Ranks     []any `json:"ranks"`
		Champions []any `json:"top_champions"`
		Recent    []any `json:"recent"`
		SoloScore int   `json:"solo_score"`
	}
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Ranks == nil || got.Champions == nil || got.Recent == nil {
		t.Error("empty lists must serialise as [] so the SPA can iterate without a guard")
	}
	if got.SoloScore != -1 {
		t.Errorf("solo_score = %d, want -1", got.SoloScore)
	}
}

func TestBuildProfileRoundTripsThroughTheStoredShape(t *testing.T) {
	blob := buildProfile("A#B", "kr",
		[]riot.RankEntry{{Queue: "RANKED_FLEX_SR", Tier: "SILVER", Division: "I", LP: 3, Wins: 1, Losses: 2, HotStreak: true}},
		[]riot.ChampionMastery{{ChampionID: 1, Name: "Annie", IconURL: "http://x/Annie.png", Level: 7, Points: 99}},
		[]riot.MatchResult{{MatchID: "KR_9", Win: false, Champion: "Annie", Kills: 1, Deaths: 2, Assists: 3,
			QueueID: 420, DurationSeconds: 1200, PlayedAt: time.Unix(1757620440, 0).UTC()}},
	)

	var got profile
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("the stored blob must unmarshal back into its own shape: %v", err)
	}
	if len(got.Ranks) != 1 || got.Ranks[0].Tier != "SILVER" || !got.Ranks[0].HotStreak {
		t.Errorf("ranks round-trip lost data: %+v", got.Ranks)
	}
	if len(got.Champions) != 1 || got.Champions[0].IconURL != "http://x/Annie.png" {
		t.Errorf("champions round-trip lost data: %+v", got.Champions)
	}
	if len(got.Recent) != 1 || got.Recent[0].Deaths != 2 || got.Recent[0].QueueID != 420 {
		t.Errorf("recent round-trip lost data: %+v", got.Recent)
	}
	if got.SoloScore != -1 {
		t.Errorf("solo_score = %d, want -1 for a flex-only account", got.SoloScore)
	}
}

func TestBuildProfileKeepsTheChampionImageID(t *testing.T) {
	blob := buildProfile("A#B", "euw1", nil,
		[]riot.ChampionMastery{{
			ChampionID: 62, Name: "Wukong", ImageID: "MonkeyKing",
			IconURL: "http://x/MonkeyKing.png", Level: 7, Points: 99,
		}}, nil)

	var got profile
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got.Champions) != 1 {
		t.Fatalf("want 1 champion, got %d", len(got.Champions))
	}
	if got.Champions[0].ImageID != "MonkeyKing" {
		t.Errorf("image_id = %q, want MonkeyKing; without it the page cannot "+
			"build the loading-screen art URL", got.Champions[0].ImageID)
	}
}
