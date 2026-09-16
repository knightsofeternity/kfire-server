package ws

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLivePayloadValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"en cours", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143,"overtime":false,"goals":1,"assists":0,"saves":2,"shots":3,"score":310,"demos":0}`, true},
		{"prolongation", `{"game_slug":"rocket-league","team_blue_score":3,"team_orange_score":3,"seconds_remaining":47,"overtime":true,"goals":1,"assists":1,"saves":0,"shots":2,"score":280,"demos":1}`, true},
		{"fin de match", `{"game_slug":"rocket-league","ended":true}`, true},
		{"slug manquant", `{"team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"score negatif", `{"game_slug":"rocket-league","team_blue_score":-1,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"chrono negatif", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":-5}`, false},
		{"chrono aberrant", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":99999}`, false},
		{"stat negative", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143,"saves":-2}`, false},
		{"score de match aberrant", `{"game_slug":"rocket-league","team_blue_score":999,"team_orange_score":1,"seconds_remaining":143}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p livePayload
			err := json.Unmarshal([]byte(tc.body), &p)
			got := err == nil && p.valid()
			if got != tc.ok {
				t.Errorf("valid() = %v, want %v", got, tc.ok)
			}
		})
	}
}

func TestLiveExpire(t *testing.T) {
	now := time.Now()
	e := liveEntry{updatedAt: now.Add(-liveTTL - time.Second)}
	if !e.expired(now) {
		t.Errorf("une entrée plus vieille que le TTL doit être expirée")
	}
	fresh := liveEntry{updatedAt: now.Add(-time.Second)}
	if fresh.expired(now) {
		t.Errorf("une entrée d'il y a une seconde ne doit pas être expirée")
	}
}
