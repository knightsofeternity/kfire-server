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

// payloadFields rend les noms JSON acceptés, dans l'ordre de déclaration. Sert
// au test qui épingle la liste de ce qui quitte la machine d'un membre.
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

		// La cohérence entre le résultat annoncé et les scores est vérifiée :
		// un client qui se déclare vainqueur en ayant perdu est refusé.
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

// body construit une charge utile terminée il y a une heure.
func body(fields string) string {
	return bodyAt(fields, time.Now().Add(-time.Hour))
}

// bodyAt construit une charge utile terminée à l'instant donné, pour que les cas
// de dérive d'horloge restent vrais quand la suite tourne.
func bodyAt(fields string, at time.Time) string {
	return fmt.Sprintf(`{%s,"played_at":%q}`, fields, at.UTC().Format(time.RFC3339))
}

func TestRecorderRevendiqueSonSlug(t *testing.T) {
	if got := NewRecorder(nil).Slug(); got != "rocket-league" {
		t.Errorf("Slug() = %q, want \"rocket-league\"", got)
	}
}

// Ce test épingle EXACTEMENT les champs que le serveur accepte de lire. En
// ajouter un sans y penser casse ici, ce qui est le but : la liste de ce qui
// quitte la machine d'un membre ne doit pas s'allonger par accident.
func TestChampsAcceptes(t *testing.T) {
	want := []string{
		"playlist", "team_size", "player_team",
		"team_blue_score", "team_orange_score", "result",
		"goals", "assists", "saves", "shots", "score", "demos",
		"mvp", "duration_seconds", "played_at",
	}
	got := payloadFields()
	if len(got) != len(want) {
		t.Fatalf("%d champs acceptés, want %d : %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("champ %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRecordRendLaSentinelleDansLesDeuxCas(t *testing.T) {
	r := NewRecorder(nil)

	jsonCasse := r.Record(context.Background(), "u1", "g1", json.RawMessage(`{`))
	if !errors.Is(jsonCasse, matchrecord.ErrInvalidPayload) {
		t.Errorf("json illisible = %v, want ErrInvalidPayload", jsonCasse)
	}
	if !strings.Contains(jsonCasse.Error(), "json illisible") {
		t.Errorf("le message doit nommer la classe d'échec, got %q", jsonCasse)
	}

	champsRefuses := r.Record(context.Background(), "u1", "g1",
		json.RawMessage(`{"playlist":73,"team_size":1,"player_team":0,"team_blue_score":0,"team_orange_score":0,"result":"draw","goals":0,"assists":0,"saves":0,"shots":0,"score":0,"demos":0,"mvp":false,"duration_seconds":60,"played_at":"2026-08-02T17:44:59Z"}`))
	if !errors.Is(champsRefuses, matchrecord.ErrInvalidPayload) {
		t.Errorf("champs refusés = %v, want ErrInvalidPayload", champsRefuses)
	}
	if !strings.Contains(champsRefuses.Error(), "Playlist:73") {
		t.Errorf("le message doit porter la playlist fautive, got %q", champsRefuses)
	}
}
