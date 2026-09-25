// Package rocketleague exposes Rocket League as a game plugin. Like
// Hearthstone, it crawls nothing: match results are reported by the desktop
// client, which reads the stats socket the game opens locally.
package rocketleague

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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

// minDuration rejects a match too short to have been played.
//
// It used to be a full minute, to refuse the phantom that clients up to
// v0.6.0-beta.3 built out of the frames the game keeps sending on the
// post-match screen (15 to 40 seconds, stopwatch restarted at the end of the
// real match). Since v0.6.0-beta.4 the client's MatchGate no longer lets a
// state frame start a match after an end, so that phantom is gone at the
// source, and the duplicate guard in InsertRocketLeagueMatch still catches an
// old client's exact copy.
//
// A minute also turned out to refuse real matches: on 24/09/2026 a member won
// by forfeit after 22 seconds of play (44 seconds measured by the client) and
// the match was dropped. Ten seconds keeps a sanity floor, below anything a
// forfeit can take, without calling a real win a phantom.
const minDuration = 10

// trainingPlaylists are the Psyonix identifiers the client must never
// report: free play, workshop maps and training. They have no opponent and
// would skew every ratio.
var trainingPlaylists = map[int]struct{}{0: {}, 9: {}, 19: {}, 21: {}, 73: {}}

// maxOthers is the most other players a report may carry: four against four,
// minus the member.
const maxOthers = 7

// matchKeyPattern is the only shape match_key may take: a SHA-256 in lowercase
// hex. The database enforces the same with a CHECK.
var matchKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// otherPlayer is one of the OTHER players of the match, as the client sends it:
// numbers and one flag, never a name.
type otherPlayer struct {
	Team    int  `json:"team"`
	Score   int  `json:"score"`
	Goals   int  `json:"goals"`
	Assists int  `json:"assists"`
	Saves   int  `json:"saves"`
	Shots   int  `json:"shots"`
	Demos   int  `json:"demos"`
	Left    bool `json:"left"`
}

// payload is a finished match, already summarised by the desktop client.
//
// The game's own stream carries the name of EVERY player in the match. The
// client uses it to compute mvp and team_size, then sends only this: facts
// about the member, plus two team scores that name no one.
//
// playlist is Psyonix's raw numeric identifier. It is never translated here:
// the identifier is the fact, the label is presentation, and it is
// localized.
//
// It is a pointer because the real protocol never sends it: UpdateStateData
// carries only MatchGuid, Players and Game, and GameState has no playlist,
// mode or ranked flag anywhere. The field is kept for a future Psyonix
// addition and for any client that still sends one, but it is optional.
type payload struct {
	Playlist        *int      `json:"playlist"`
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
	// MatchKey and Others arrive together or not at all, from clients that
	// build the scoreboard. Older clients send neither and are unaffected.
	MatchKey *string       `json:"match_key"`
	Others   []otherPlayer `json:"others"`
}

// String renders the payload for the log. It exists only because Playlist
// is now a pointer: without it, %+v would print an address instead of the
// value, which is useless to an operator reading the log after a rejection.
func (p payload) String() string {
	playlist := "<nil>"
	if p.Playlist != nil {
		playlist = fmt.Sprintf("%d", *p.Playlist)
	}
	key := "<nil>"
	if p.MatchKey != nil {
		key = *p.MatchKey
	}
	return fmt.Sprintf(
		"{Playlist:%s TeamSize:%d PlayerTeam:%d TeamBlueScore:%d TeamOrangeScore:%d "+
			"Result:%s Goals:%d Assists:%d Saves:%d Shots:%d Score:%d Demos:%d "+
			"MVP:%t DurationSeconds:%d PlayedAt:%v MatchKey:%s Others:%+v}",
		playlist, p.TeamSize, p.PlayerTeam, p.TeamBlueScore, p.TeamOrangeScore,
		p.Result, p.Goals, p.Assists, p.Saves, p.Shots, p.Score, p.Demos,
		p.MVP, p.DurationSeconds, p.PlayedAt, key, p.Others)
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
	// An absent playlist is valid: the real protocol never sends one. A
	// present playlist must still be a sane, non-training identifier.
	if p.Playlist != nil {
		if *p.Playlist < 0 {
			return false
		}
		if _, training := trainingPlaylists[*p.Playlist]; training {
			return false
		}
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
	if p.DurationSeconds < minDuration || p.DurationSeconds > maxDuration {
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
	if !p.validScoreboard() {
		return false
	}
	return true
}

// validScoreboard checks the scoreboard half of a payload.
//
// The key and the players come together or not at all, so a stored match with
// a key always has its players and the page never offers an empty scoreboard.
// Each team may hold at most team_size players still present at the end,
// counting the member: a player who left does not count, since a substitute
// may have taken their place.
func (p payload) validScoreboard() bool {
	if p.MatchKey == nil && p.Others == nil {
		return true
	}
	if p.MatchKey == nil || p.Others == nil {
		return false
	}
	if !matchKeyPattern.MatchString(*p.MatchKey) {
		return false
	}
	if len(p.Others) < 1 || len(p.Others) > maxOthers {
		return false
	}
	present := [2]int{}
	present[p.PlayerTeam]++
	for _, o := range p.Others {
		if o.Team != 0 && o.Team != 1 {
			return false
		}
		if o.Score < 0 || o.Goals < 0 || o.Assists < 0 || o.Saves < 0 ||
			o.Shots < 0 || o.Demos < 0 {
			return false
		}
		if !o.Left {
			present[o.Team]++
		}
	}
	return present[0] <= p.TeamSize && present[1] <= p.TeamSize
}

// Recorder writes the Rocket League matches reported by the desktop client.
type Recorder struct {
	st *store.Store
}

// NewRecorder builds the recorder.
func NewRecorder(st *store.Store) *Recorder { return &Recorder{st: st} }

// Slug returns the claimed catalog slug.
func (r *Recorder) Slug() string { return SLUG }

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
	var others []store.RocketLeaguePlayer
	for _, o := range p.Others {
		others = append(others, store.RocketLeaguePlayer{
			Team: o.Team, Score: o.Score, Goals: o.Goals, Assists: o.Assists,
			Saves: o.Saves, Shots: o.Shots, Demos: o.Demos, Left: o.Left,
		})
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
		MatchKey: p.MatchKey, Others: others,
	})
}
