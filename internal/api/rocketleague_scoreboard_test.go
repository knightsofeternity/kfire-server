package api

import (
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/rocketleague"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func TestScoreboardJSON(t *testing.T) {
	sb := rocketleague.Scoreboard{ReferenceTeam: 1}
	sb.Teams[0] = rocketleague.ScoreTeam{Team: 0, Score: 4, Winner: true, Rows: []rocketleague.ScoreRow{
		{AnonIndex: 1, Player: store.RocketLeaguePlayer{Team: 0, Score: 583}, MVP: true},
	}}
	sb.Teams[1] = rocketleague.ScoreTeam{Team: 1, Score: 2, Rows: []rocketleague.ScoreRow{
		{UserID: "u1", Username: "DonZeZe", Player: store.RocketLeaguePlayer{Team: 1, Score: 360, Goals: 1}},
	}}
	out := scoreboardJSON("m-1", sb)
	if out["match_id"] != "m-1" || out["reference_team"] != 1 {
		t.Fatalf("en-tete inattendu : %v", out)
	}
	teams := out["teams"].([]map[string]any)
	anon := teams[0]["rows"].([]map[string]any)[0]
	if _, named := anon["username"]; named {
		t.Fatal("une ligne anonyme ne doit porter aucun champ de nom")
	}
	if anon["anon_index"] != 1 || anon["mvp"] != true {
		t.Fatalf("ligne anonyme inattendue : %v", anon)
	}
	member := teams[1]["rows"].([]map[string]any)[0]
	if member["username"] != "DonZeZe" || member["goals"] != 1 {
		t.Fatalf("ligne membre inattendue : %v", member)
	}
	if _, anonIdx := member["anon_index"]; anonIdx {
		t.Fatal("un membre nomme ne porte pas d anon_index")
	}
}
