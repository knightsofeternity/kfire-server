// Package riotsync refreshes a member's League of Legends data on demand and
// tracks who is in a game right now.
package riotsync

import "github.com/knightsofeternity/kfire-server/internal/connectors/riot"

// soloQueue is the queue the guild leaderboard ranks on.
const soloQueue = "RANKED_SOLO_5x5"

var tierWeight = map[string]int{
	"IRON": 0, "BRONZE": 1, "SILVER": 2, "GOLD": 3, "PLATINUM": 4,
	"EMERALD": 5, "DIAMOND": 6, "MASTER": 7, "GRANDMASTER": 8, "CHALLENGER": 9,
}

var divisionWeight = map[string]int{"IV": 0, "III": 1, "II": 2, "I": 3}

// SoloScore collapses a solo-queue standing into one sortable integer, so the
// guild leaderboard orders members with a plain numeric comparison. Unranked,
// or ranked in flex only, scores -1 and therefore sorts last.
//
// The tier stride is 100000 and the division stride 10000, both far above the
// highest LP total seen in Master and above, where LP is unbounded in practice
// but stays in the low thousands. A high-LP Master player can therefore never
// outscore a Grandmaster.
//
// A tier Riot adds later, absent from tierWeight, also scores -1: sorting an
// unknown tier last is safer than silently scoring it as iron.
func SoloScore(entries []riot.RankEntry) int {
	for _, e := range entries {
		if e.Queue != soloQueue {
			continue
		}
		tier, ok := tierWeight[e.Tier]
		if !ok {
			return -1
		}
		return tier*100000 + divisionWeight[e.Division]*10000 + e.LP
	}
	return -1
}
