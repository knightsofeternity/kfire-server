package hearthstone

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
// a match result is rejected.
const clockSkewTolerance = 5 * time.Minute

// payload is a finished match, already summarised by the desktop client. The
// client reads the game's log files and sends only this: never the
// opponent's name, never a card.
//
// Turns and Placement are pointers because a match deserves to be recorded
// even when a secondary field was unreadable.
type payload struct {
	Mode      string `json:"mode"`
	Result    string `json:"result"`
	Turns     *int   `json:"turns"`
	Placement *int   `json:"placement"`
	// HeroCardID is the Battlegrounds hero, as the card identifier the game
	// writes in its own log. Never the hero's name: the log is localized, so
	// two members playing the same hero would report two different strings.
	HeroCardID *string   `json:"hero_card_id"`
	PlayedAt   time.Time `json:"played_at"`
}

// heroCardID is the shape of a card identifier. Rejecting anything else keeps
// the column incapable of carrying a name, which is the whole point.
var heroCardID = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// valid reports whether the payload deserves to be written. The database
// enforces the same rules, but rejecting here gives the client a clear error
// instead of an opaque write failure.
//
// The slug is NOT checked here: that belongs to the routing, not to the game.
func (p payload) valid() bool {
	if p.PlayedAt.IsZero() {
		return false
	}
	if p.Mode != "battlegrounds" && p.Mode != "constructed" {
		return false
	}
	if p.Result != "win" && p.Result != "loss" && p.Result != "draw" {
		return false
	}
	if p.Placement != nil && (*p.Placement < 1 || *p.Placement > 8) {
		return false
	}
	if p.Turns != nil && *p.Turns < 0 {
		return false
	}
	if p.HeroCardID != nil && !heroCardID.MatchString(*p.HeroCardID) {
		return false
	}
	// A queued match can be old, never future. The tolerance absorbs a
	// desktop clock that drifts a little without letting a badly set clock
	// poison the whole roster's last-played date.
	if p.PlayedAt.After(time.Now().Add(clockSkewTolerance)) {
		return false
	}
	return true
}

// Recorder writes the Hearthstone matches reported by the desktop client.
type Recorder struct {
	st *store.Store
}

// NewRecorder builds the recorder.
func NewRecorder(st *store.Store) *Recorder { return &Recorder{st: st} }

// Slug returns the claimed catalog slug.
func (r *Recorder) Slug() string { return "hearthstone" }

// Record validates then writes a match.
//
// The two failure classes are distinguished in the message, while staying the
// same sentinel error: the hub only exposes a generic code to the client, but
// logs the detail. An unreadable JSON means the client's serialization is
// broken and ALL of its matches will fail; a validation rejection often means
// legitimate data ran into a rule written too early, like a game mode that
// did not exist yet. Without the detail in the log, the two look alike and
// neither is diagnosable.
func (r *Recorder) Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("%w: unreadable json: %v", matchrecord.ErrInvalidPayload, err)
	}
	if !p.valid() {
		// mode and result are the discriminating fields: if a new mode
		// appears one day, it reads directly from the log.
		return fmt.Errorf("%w: rejected fields (mode=%q result=%q)",
			matchrecord.ErrInvalidPayload, p.Mode, p.Result)
	}
	return r.st.InsertHearthstoneMatch(ctx, store.HearthstoneMatch{
		UserID: userID, GameID: gameID,
		Mode: p.Mode, Result: p.Result,
		Turns: p.Turns, Placement: p.Placement, HeroCardID: p.HeroCardID,
		PlayedAt: p.PlayedAt,
	})
}
