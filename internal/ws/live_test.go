package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
		{"slug avec du html", `{"game_slug":"<script>alert(1)</script>","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"slug a rallonge", fmt.Sprintf(`{"game_slug":%q,"team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, strings.Repeat("a", 200)), false},
		{"slug majuscule", `{"game_slug":"Rocket-League","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143}`, false},
		{"stat aberrante", `{"game_slug":"rocket-league","team_blue_score":2,"team_orange_score":1,"seconds_remaining":143,"score":2147483647}`, false},
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

func TestLiveEtatPartage(t *testing.T) {
	// Un hub sans dépendances suffit : setLive et LiveMatch ne touchent que la
	// carte en mémoire. C'est ce qui rend cet état testable alors que le reste
	// du hub ne l'est pas.
	h := NewHub(nil, nil, "", nil)

	p := livePayload{
		GameSlug: "rocket-league", TeamBlueScore: 2, TeamOrangeScore: 1,
		SecondsRemaining: 143, Goals: 1, Saves: 2, Shots: 3, Score: 310,
	}
	h.setLive("u1", p)

	got := h.LiveMatch("u1")
	if got == nil {
		t.Fatal("LiveMatch rend nil juste après setLive")
	}
	if got["team_blue_score"] != 2 || got["seconds_remaining"] != 143 {
		t.Errorf("état relu incorrect : %v", got)
	}
	if _, nomme := got["user_id"]; nomme {
		t.Error("l'état diffusé ne doit pas porter d'identité, le hub l'ajoute autour")
	}

	// Un membre sans match en cours n'a pas d'état.
	if h.LiveMatch("inconnu") != nil {
		t.Error("LiveMatch rend un état pour un membre qui ne joue pas")
	}

	// La fin de match efface.
	h.setLive("u1", livePayload{GameSlug: "rocket-league", Ended: true})
	if h.LiveMatch("u1") != nil {
		t.Error("l'état survit à la fin du match")
	}
}

// Ce test n'a de valeur que lancé avec -race : il fait se croiser lectures et
// écritures sur la carte partagée, ce qu'aucun autre test du paquet ne fait.
// Sans lui, `go test -race ./internal/ws/` ne prouve rien sur h.live.
func TestLiveAccesConcurrent(t *testing.T) {
	h := NewHub(nil, nil, "", nil)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		membre := fmt.Sprintf("u%d", i%3)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.setLive(membre, livePayload{GameSlug: "rocket-league", TeamBlueScore: j % 5})
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = h.LiveMatch(membre)
			}
		}()
	}
	wg.Wait()
}

func TestLiveVisibiliteCoupeLeDirect(t *testing.T) {
	h := NewHub(nil, nil, "", nil)
	h.setLiveVisible("u1", true, "online")
	if !h.liveAllowed("u1") {
		t.Fatal("un membre visible doit pouvoir diffuser")
	}
	h.setLive("u1", livePayload{GameSlug: "rocket-league", TeamBlueScore: 1})

	// Il se cache en pleine partie.
	h.SetVisibility("u1", true, "invisible")
	if h.liveAllowed("u1") {
		t.Error("un membre invisible ne doit plus diffuser")
	}
	if h.LiveMatch("u1") != nil {
		t.Error("son match en direct devait disparaître immédiatement")
	}

	// Un membre qui masque son activité sans se déclarer invisible, aussi.
	h.setLiveVisible("u2", false, "online")
	if h.liveAllowed("u2") {
		t.Error("activité masquée doit suffire à couper le direct")
	}
}

// La bascule de visibilité arrive par un fil HTTP pendant que la boucle de
// lecture de la connexion consulte l'autorisation. C'est exactement le
// croisement qui rendait la première version de ce correctif inacceptable.
func TestLiveVisibiliteAccesConcurrent(t *testing.T) {
	h := NewHub(nil, nil, "", nil)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				h.SetVisibility("u1", j%2 == 0, "online")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				if h.liveAllowed("u1") {
					h.setLive("u1", livePayload{GameSlug: "rocket-league"})
				}
			}
		}()
	}
	wg.Wait()
}
