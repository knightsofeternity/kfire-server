package riotsync

import (
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
)

func TestSoloScoreOrdersTiersThenDivisionsThenLP(t *testing.T) {
	ordered := []riot.RankEntry{
		{Queue: "RANKED_SOLO_5x5", Tier: "IRON", Division: "IV", LP: 0},
		{Queue: "RANKED_SOLO_5x5", Tier: "IRON", Division: "I", LP: 99},
		{Queue: "RANKED_SOLO_5x5", Tier: "GOLD", Division: "IV", LP: 0},
		{Queue: "RANKED_SOLO_5x5", Tier: "GOLD", Division: "II", LP: 47},
		{Queue: "RANKED_SOLO_5x5", Tier: "EMERALD", Division: "IV", LP: 0},
		{Queue: "RANKED_SOLO_5x5", Tier: "CHALLENGER", Division: "I", LP: 1500},
	}
	for i := 1; i < len(ordered); i++ {
		lo := SoloScore([]riot.RankEntry{ordered[i-1]})
		hi := SoloScore([]riot.RankEntry{ordered[i]})
		if lo >= hi {
			t.Errorf("%+v scored %d, %+v scored %d; want strictly increasing",
				ordered[i-1], lo, ordered[i], hi)
		}
	}
}

func TestSoloScoreIgnoresFlex(t *testing.T) {
	entries := []riot.RankEntry{
		{Queue: "RANKED_FLEX_SR", Tier: "CHALLENGER", Division: "I", LP: 900},
	}
	if got := SoloScore(entries); got != -1 {
		t.Errorf("SoloScore with flex only = %d, want -1", got)
	}
}

func TestSoloScorePicksSoloOutOfAMixedList(t *testing.T) {
	entries := []riot.RankEntry{
		{Queue: "RANKED_FLEX_SR", Tier: "CHALLENGER", Division: "I", LP: 900},
		{Queue: "RANKED_SOLO_5x5", Tier: "SILVER", Division: "III", LP: 12},
	}
	want := SoloScore([]riot.RankEntry{{Queue: "RANKED_SOLO_5x5", Tier: "SILVER", Division: "III", LP: 12}})
	if got := SoloScore(entries); got != want {
		t.Errorf("SoloScore = %d, want %d; flex must not shadow solo whatever the order", got, want)
	}
}

func TestSoloScoreUnrankedIsMinusOne(t *testing.T) {
	if got := SoloScore(nil); got != -1 {
		t.Errorf("SoloScore(nil) = %d, want -1", got)
	}
}

func TestSoloScoreUnknownTierIsMinusOne(t *testing.T) {
	entries := []riot.RankEntry{{Queue: "RANKED_SOLO_5x5", Tier: "MYTHIC", Division: "I", LP: 10}}
	if got := SoloScore(entries); got != -1 {
		t.Errorf("SoloScore with an unknown tier = %d, want -1; a tier Riot adds later "+
			"must sort last rather than score as iron", got)
	}
}

func TestSoloScoreMasterTierLPCannotSpillIntoTheNextTier(t *testing.T) {
	master := SoloScore([]riot.RankEntry{
		{Queue: "RANKED_SOLO_5x5", Tier: "MASTER", Division: "I", LP: 9999},
	})
	grandmaster := SoloScore([]riot.RankEntry{
		{Queue: "RANKED_SOLO_5x5", Tier: "GRANDMASTER", Division: "I", LP: 0},
	})
	if master >= grandmaster {
		t.Errorf("master at 9999 LP scored %d, grandmaster at 0 LP scored %d", master, grandmaster)
	}
}
