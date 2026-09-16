// Package rocketleague exposes Rocket League as a game plugin. Like
// Hearthstone, it crawls nothing: match results are reported by the desktop
// client, which reads the stats socket the game opens locally.
package rocketleague

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// clockSkewTolerance is the maximum lead tolerated on the server clock before
// a match result is rejected. Same value and same reason as for Hearthstone.
const clockSkewTolerance = 5 * time.Minute

// maxDuration bounds the duration of a match. Overtime can drag on, but a
// runaway clock must not poison the guild's play time.
//
// MUST stay equal to the duration_seconds ceiling in migration 0035. If the
// two diverge, a payload passes validation then gets rejected by the
// constraint, the hub classes it as a transient error, and the client
// re-emits it forever.
const maxDuration = 7200

// trainingPlaylists are the Psyonix identifiers the client must never
// report: free play, workshop maps and training. They have no opponent and
// would skew every ratio.
var trainingPlaylists = map[int]struct{}{0: {}, 9: {}, 19: {}, 21: {}, 73: {}}

// payload is a finished match, already summarised by the desktop client.
//
// The game's own stream carries the name of EVERY player in the match. The
// client uses it to compute mvp and team_size, then sends only this: facts
// about the member, plus two team scores that name no one.
//
// playlist is Psyonix's raw numeric identifier. It is never translated here:
// the identifier is the fact, the label is presentation, and it is
// localized.
type payload struct {
	Playlist        int       `json:"playlist"`
	TeamSize        int       `json:"team_size"`
	PlayerTeam      int       `json:"player_team"`
	TeamBlueScore   int       `json:"team_blue_score"`
	TeamOrangeScore int       `json:"team_orange_score"`
	Result          string    `json:"result"`
	Goals           int       `json:"goals"`
	Assists         int       `json:"assists"`
	Saves           int       `json:"saves"`
	Shots           int       `json:"shots"`
	Score           int       `json:"score"`
	Demos           int       `json:"demos"`
	MVP             bool      `json:"mvp"`
	DurationSeconds int       `json:"duration_seconds"`
	PlayedAt        time.Time `json:"played_at"`
}

// expectedResult returns the result the scores impose, from the member's
// team's point of view.
func expectedResult(playerTeam, blue, orange int) string {
	mine, theirs := blue, orange
	if playerTeam == 1 {
		mine, theirs = orange, blue
	}
	switch {
	case mine > theirs:
		return "win"
	case mine < theirs:
		return "loss"
	default:
		return "draw"
	}
}

// valid reports whether the payload deserves to be written. The database
// enforces the same bounds, but rejecting here gives the client a clear
// error instead of an opaque write failure, and allows two checks SQL cannot
// do: consistency between the announced result and the scores, and
// rejecting an MVP without a win.
func (p payload) valid() bool {
	if p.PlayedAt.IsZero() {
		return false
	}
	if p.Playlist < 0 {
		return false
	}
	if _, training := trainingPlaylists[p.Playlist]; training {
		return false
	}
	if p.TeamSize < 1 || p.TeamSize > 4 {
		return false
	}
	if p.PlayerTeam != 0 && p.PlayerTeam != 1 {
		return false
	}
	if p.TeamBlueScore < 0 || p.TeamOrangeScore < 0 {
		return false
	}
	if p.Goals < 0 || p.Assists < 0 || p.Saves < 0 ||
		p.Shots < 0 || p.Score < 0 || p.Demos < 0 {
		return false
	}
	if p.DurationSeconds < 0 || p.DurationSeconds > maxDuration {
		return false
	}
	// The announced result must match the scores: a client does not declare
	// itself the winner of a match it lost.
	if p.Result != expectedResult(p.PlayerTeam, p.TeamBlueScore, p.TeamOrangeScore) {
		return false
	}
	// The MVP is the best score on the WINNING team: it cannot exist without
	// a win.
	if p.MVP && p.Result != "win" {
		return false
	}
	// A queued match can be old, never future.
	if p.PlayedAt.After(time.Now().Add(clockSkewTolerance)) {
		return false
	}
	return true
}

// Recorder writes the Rocket League matches reported by the desktop client.
type Recorder struct {
	st *store.Store
}

// NewRecorder builds the recorder.
func NewRecorder(st *store.Store) *Recorder { return &Recorder{st: st} }

// Slug returns the claimed catalog slug.
func (r *Recorder) Slug() string { return "rocket-league" }

// Record validates then writes a match.
//
// Same error envelope as the Hearthstone recorder: one sentinel on the wire,
// the detail in the log. playlist is the discriminating field, since an
// unknown Psyonix identifier is the most likely reason for a legitimate
// rejection.
func (r *Recorder) Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("%w: unreadable json: %v", matchrecord.ErrInvalidPayload, err)
	}
	if !p.valid() {
		// The WHOLE payload goes to the log, and that is safe by
		// construction: every field is a number, a boolean or an enum, by
		// the same design that makes the table incapable of holding a
		// pseudonym. There is nothing personal in it to leak.
		//
		// Naming only two of them would force the operator to replay each
		// rule of valid() by hand against a payload that, itself, was not
		// logged.
		return fmt.Errorf("%w: rejected fields (%+v)", matchrecord.ErrInvalidPayload, p)
	}
	return r.st.InsertRocketLeagueMatch(ctx, store.RocketLeagueMatch{
		UserID: userID, GameID: gameID,
		Playlist: p.Playlist, TeamSize: p.TeamSize, PlayerTeam: p.PlayerTeam,
		TeamBlueScore: p.TeamBlueScore, TeamOrangeScore: p.TeamOrangeScore,
		Result: p.Result,
		Goals:  p.Goals, Assists: p.Assists, Saves: p.Saves,
		Shots: p.Shots, Score: p.Score, Demos: p.Demos,
		MVP: p.MVP, DurationSeconds: p.DurationSeconds,
		PlayedAt: p.PlayedAt,
	})
}
