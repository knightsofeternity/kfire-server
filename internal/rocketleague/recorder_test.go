package rocketleague

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
)

// payloadFields returns the accepted JSON names, in declaration order. Used
// by the test that pins down the list of what leaves a member's machine.
func payloadFields() []string {
	t := reflect.TypeOf(payload{})
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out = append(out, t.Field(i).Tag.Get("json"))
	}
	return out
}

func TestPayloadValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"victoire bleue complete", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":2,"assists":1,"saves":3,"shots":5,"score":640,"demos":1,"mvp":true,"duration_seconds":330`), true},
		{"defaite orange", body(`"playlist":11,"team_size":2,"player_team":1,"team_blue_score":5,"team_orange_score":1,"result":"loss","goals":0,"assists":0,"saves":1,"shots":2,"score":180,"demos":0,"mvp":false,"duration_seconds":300`), true},
		{"victoire orange", body(`"playlist":13,"team_size":3,"player_team":1,"team_blue_score":2,"team_orange_score":5,"result":"win","goals":3,"assists":1,"saves":0,"shots":4,"score":520,"demos":2,"mvp":true,"duration_seconds":345`), true},
		{"match nul", body(`"playlist":6,"team_size":3,"player_team":0,"team_blue_score":2,"team_orange_score":2,"result":"draw","goals":1,"assists":0,"saves":0,"shots":3,"score":250,"demos":0,"mvp":false,"duration_seconds":300`), true},

		{"entrainement refuse", body(`"playlist":73,"team_size":1,"player_team":0,"team_blue_score":0,"team_orange_score":0,"result":"draw","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":60`), false},
		{"partie libre refusee", body(`"playlist":0,"team_size":1,"player_team":0,"team_blue_score":0,"team_orange_score":0,"result":"draw","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":60`), false},

		{"equipe hors domaine", body(`"playlist":13,"team_size":3,"player_team":2,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"taille d equipe nulle", body(`"playlist":13,"team_size":0,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"taille d equipe trop grande", body(`"playlist":13,"team_size":5,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"score negatif", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":-1,"team_orange_score":2,"result":"loss","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"buts negatifs", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":-1,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"resultat inconnu", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"maybe","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"duree aberrante", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":99999`), false},

		// Consistency between the announced result and the scores is
		// checked: a client that declares itself the winner while it lost
		// is rejected.
		{"victoire annoncee mais score perdant", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":1,"team_orange_score":4,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"nul annonce mais scores differents", body(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":3,"team_orange_score":1,"result":"draw","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`), false},
		{"mvp sans victoire", body(`"playlist":13,"team_size":3,"player_team":1,"team_blue_score":4,"team_orange_score":2,"result":"loss","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":true,"duration_seconds":300`), false},

		{"date manquante", `{"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300}`, false},
		{"date future", bodyAt(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`, time.Now().Add(time.Hour)), false},
		{"date a peine future, tolerance d horloge", bodyAt(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`, time.Now().Add(time.Minute)), true},
		{"date passee", bodyAt(`"playlist":13,"team_size":3,"player_team":0,"team_blue_score":4,"team_orange_score":2,"result":"win","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":300`, time.Now().Add(-48*time.Hour)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p payload
			err := json.Unmarshal([]byte(tc.body), &p)
			got := err == nil && p.valid()
			if got != tc.ok {
				t.Errorf("valid() = %v, want %v", got, tc.ok)
			}
		})
	}
}

// body builds a payload that ended one hour ago.
func body(fields string) string {
	return bodyAt(fields, time.Now().Add(-time.Hour))
}

// bodyAt builds a payload that ended at the given instant, so the
// clock-skew cases stay true whenever the suite runs.
func bodyAt(fields string, at time.Time) string {
	return fmt.Sprintf(`{%s,"played_at":%q}`, fields, at.UTC().Format(time.RFC3339))
}

func TestRecorderClaimsItsSlug(t *testing.T) {
	if got := NewRecorder(nil).Slug(); got != "rocket-league" {
		t.Errorf("Slug() = %q, want \"rocket-league\"", got)
	}
}

// This test pins down EXACTLY the fields the server agrees to read. Adding
// one without thinking about it breaks here, which is the point: the list of
// what leaves a member's machine must not grow by accident.
func TestAcceptedFields(t *testing.T) {
	want := []string{
		"playlist", "team_size", "player_team",
		"team_blue_score", "team_orange_score", "result",
		"goals", "assists", "saves", "shots", "score", "demos",
		"mvp", "duration_seconds", "played_at",
	}
	got := payloadFields()
	if len(got) != len(want) {
		t.Fatalf("%d accepted fields, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRecordReturnsTheSentinelInBothCases(t *testing.T) {
	r := NewRecorder(nil)

	malformedJSON := r.Record(context.Background(), "u1", "g1", json.RawMessage(`{`))
	if !errors.Is(malformedJSON, matchrecord.ErrInvalidPayload) {
		t.Errorf("unreadable json = %v, want ErrInvalidPayload", malformedJSON)
	}
	if !strings.Contains(malformedJSON.Error(), "unreadable json") {
		t.Errorf("message should name the failure class, got %q", malformedJSON)
	}

	rejectedFields := r.Record(context.Background(), "u1", "g1",
		json.RawMessage(`{"playlist":73,"team_size":1,"player_team":0,"team_blue_score":0,"team_orange_score":0,"result":"draw","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":60,"played_at":"2026-08-02T17:44:59Z"}`))
	if !errors.Is(rejectedFields, matchrecord.ErrInvalidPayload) {
		t.Errorf("rejected fields = %v, want ErrInvalidPayload", rejectedFields)
	}
	if !strings.Contains(rejectedFields.Error(), "Playlist:73") {
		t.Errorf("message should carry the offending playlist, got %q", rejectedFields)
	}
}
