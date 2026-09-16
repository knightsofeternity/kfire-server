package ws

import (
	"regexp"
	"time"
)

// liveTTL is how long a match state with no new sample is considered
// finished. The client emits at 2 Hz, so 15 seconds leaves comfortable
// margin for a connection that hiccups.
const liveTTL = 15 * time.Second

// maxMatchScore bounds a team score. Rocket League has no hard limit, but a
// match with more than 99 goals does not exist: the bound stops a hostile
// client from broadcasting anything to the whole guild.
const maxMatchScore = 99

// maxSecondsRemaining bounds the displayed clock, overtime included.
const maxSecondsRemaining = 7200

// maxStat bounds the member's stats. Deliberately generous: this is not a
// game rule, it is a guard rail against an absurd value broadcast to
// everyone. The team scores and the clock were already bounded, these were
// not, for no reason.
const maxStat = 100000

// slugPattern is the shape of a catalog slug.
//
// This message is the ONLY one in the whole feature whose content is
// relayed to other members. Everything else ends up in the database,
// protected by constraints. Here, a free-form string would go straight to
// every browser in the guild as-is, so it is bounded in both length AND
// alphabet: a slug can contain no tag, no space, no ten megabytes.
//
// Resolving the slug against the catalog would be stricter, but this
// message arrives twice a second per player: hitting the database at that
// rate to validate a constant would make no sense.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// livePayload is the current state of a match, broadcast and NEVER written.
//
// Like the end-of-match summary, it names no one: two team scores, a clock,
// and the member's own stats. The game's own stream carries the name of
// every player; they never leave its machine, not even for a display.
type livePayload struct {
	GameSlug string `json:"game_slug"`
	// Ended marks the end of the match. Every other field is then ignored.
	Ended            bool `json:"ended"`
	TeamBlueScore    int  `json:"team_blue_score"`
	TeamOrangeScore  int  `json:"team_orange_score"`
	SecondsRemaining int  `json:"seconds_remaining"`
	Overtime         bool `json:"overtime"`
	Goals            int  `json:"goals"`
	Assists          int  `json:"assists"`
	Saves            int  `json:"saves"`
	Shots            int  `json:"shots"`
	Score            int  `json:"score"`
	Demos            int  `json:"demos"`
}

// valid reports whether the state deserves to be rebroadcast. It goes out to
// every client in the org, so it is validated with the same rigor as
// written data.
func (p livePayload) valid() bool {
	if !slugPattern.MatchString(p.GameSlug) {
		return false
	}
	if p.Ended {
		return true
	}
	if p.TeamBlueScore < 0 || p.TeamBlueScore > maxMatchScore ||
		p.TeamOrangeScore < 0 || p.TeamOrangeScore > maxMatchScore {
		return false
	}
	if p.SecondsRemaining < 0 || p.SecondsRemaining > maxSecondsRemaining {
		return false
	}
	for _, v := range []int{p.Goals, p.Assists, p.Saves, p.Shots, p.Score, p.Demos} {
		if v < 0 || v > maxStat {
			return false
		}
	}
	return true
}

// liveEntry is the state kept in memory for a member.
type liveEntry struct {
	payload   livePayload
	updatedAt time.Time
}

// expired reports whether the entry has gone without a new sample long
// enough to be considered finished.
func (e liveEntry) expired(now time.Time) bool {
	return now.Sub(e.updatedAt) > liveTTL
}
