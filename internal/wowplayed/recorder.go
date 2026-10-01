// Package wowplayed records the World of Warcraft /played the KFire addon
// reads in game. The desktop client sends it as a match_result whose
// game_slug is the edition, so it rides the existing persistent queue; the
// matchrecord registry routes it here.
package wowplayed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// Slugs are the catalog editions the addon runs in.
var Slugs = []string{"world-of-warcraft", "world-of-warcraft-classic", "world-of-warcraft-forever", "wow-ascension"}

const (
	maxCharacters = 200
	maxPlayed     = 25 * 365 * 24 * 3600 // no character has been played longer than WoW has existed
	clockSkew     = 5 * time.Minute
	provider      = "wow_addon"
)

var regions = map[string]bool{"us": true, "eu": true, "kr": true, "tw": true, "cn": true, "unknown": true}

// Recorder handles one edition.
type Recorder struct {
	st   *store.Store
	slug string
}

// Recorders returns one recorder per edition, for the registry.
func Recorders(st *store.Store) []matchrecord.Recorder {
	out := make([]matchrecord.Recorder, 0, len(Slugs))
	for _, s := range Slugs {
		out = append(out, &Recorder{st: st, slug: s})
	}
	return out
}

func (r *Recorder) Slug() string { return r.slug }

type character struct {
	Region        string    `json:"region"`
	Realm         string    `json:"realm"`
	RealmNorm     string    `json:"realm_norm"`
	Name          string    `json:"name"`
	PlayedSeconds int64     `json:"played_seconds"`
	Level         *int      `json:"level"`
	Class         *string   `json:"class"`
	RecordedAt    time.Time `json:"recorded_at"`
}

type payload struct {
	Characters []character `json:"characters"`
}

func textOK(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max && utf8.ValidString(s) && !strings.ContainsAny(s, "\x00\n\r")
}

// valid reports whether one character deserves to be written.
func (c character) valid(now time.Time) bool {
	if !regions[c.Region] || !textOK(c.Name, 2, 24) || !textOK(c.Realm, 1, 64) || !textOK(c.RealmNorm, 1, 64) {
		return false
	}
	if c.PlayedSeconds < 0 || c.PlayedSeconds > maxPlayed {
		return false
	}
	if c.RecordedAt.IsZero() || c.RecordedAt.After(now.Add(clockSkew)) {
		return false
	}
	if c.Level != nil && (*c.Level < 1 || *c.Level > 200) {
		return false
	}
	if c.Class != nil && !textOK(*c.Class, 1, 32) {
		return false
	}
	return true
}

// accepted keeps the valid characters of a payload. An invalid one is dropped,
// not the whole list: one odd character must not cost a member every other.
func accepted(raw json.RawMessage, now time.Time) ([]store.WowPlayed, error) {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("%w: unreadable json: %v", matchrecord.ErrInvalidPayload, err)
	}
	if len(p.Characters) == 0 || len(p.Characters) > maxCharacters {
		return nil, fmt.Errorf("%w: %d characters", matchrecord.ErrInvalidPayload, len(p.Characters))
	}
	var out []store.WowPlayed
	for _, c := range p.Characters {
		if !c.valid(now) {
			continue
		}
		out = append(out, store.WowPlayed{
			Region: c.Region, RealmNorm: c.RealmNorm, Realm: c.Realm, Name: c.Name,
			PlayedSeconds: c.PlayedSeconds, Level: c.Level, Class: c.Class, RecordedAt: c.RecordedAt.UTC(),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no valid character among %d", matchrecord.ErrInvalidPayload, len(p.Characters))
	}
	return out, nil
}

// Record stores the characters, then the edition's total as the member's
// platform playtime: baseline plus later sessions, like Steam, so the hours
// the client tracks after the snapshot add up without being counted twice.
func (r *Recorder) Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error {
	chars, err := accepted(raw, time.Now())
	if err != nil {
		return err
	}
	if err := r.st.UpsertWowPlayed(ctx, userID, gameID, chars); err != nil {
		return err
	}
	total, err := r.st.WowPlayedTotal(ctx, userID, gameID)
	if err != nil {
		return err
	}
	return r.st.UpsertExternalPlaytime(ctx, userID, provider, gameID, total)
}
