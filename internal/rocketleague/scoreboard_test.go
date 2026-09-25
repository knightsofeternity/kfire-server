package rocketleague

import (
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

func line(team, score, goals int) store.RocketLeaguePlayer {
	return store.RocketLeaguePlayer{Team: team, Score: score, Goals: goals, Assists: 1, Saves: 1, Shots: 2}
}

func report(id, name string, visible bool, l store.RocketLeaguePlayer) store.RocketLeagueReport {
	return store.RocketLeagueReport{
		MatchID: "m-" + id, UserID: id, Username: name, Visible: visible,
		TeamBlueScore: 2, TeamOrangeScore: 4, Line: l,
	}
}

// Trois membres chez les orange, gagnants 4-2, trois inconnus chez les bleus.
func TestTableauTroisMembres(t *testing.T) {
	don := report("u1", "DonZeZe", true, line(1, 360, 1))
	osi := report("u2", "Osiris", true, line(1, 238, 1))
	joul := report("u3", "Joul", true, line(1, 124, 0))
	others := []store.RocketLeaguePlayer{
		line(0, 583, 0), line(0, 531, 3), line(0, 296, 1),
		osi.Line, joul.Line,
	}
	sb := BuildScoreboard(don, others, []store.RocketLeagueReport{joul, osi})

	if sb.ReferenceTeam != 1 {
		t.Fatalf("equipe de reference %d, attendu 1", sb.ReferenceTeam)
	}
	orange := sb.Teams[1]
	if orange.Score != 4 || !orange.Winner || sb.Teams[0].Winner {
		t.Fatalf("scores ou vainqueur inattendus : %+v", sb.Teams)
	}
	names := []string{orange.Rows[0].Username, orange.Rows[1].Username, orange.Rows[2].Username}
	if names[0] != "DonZeZe" || names[1] != "Osiris" || names[2] != "Joul" {
		t.Fatalf("membres mal places : %v", names)
	}
	if !orange.Rows[0].MVP {
		t.Fatal("le meilleur score de l equipe gagnante doit etre MVP")
	}
	blue := sb.Teams[0]
	for i, r := range blue.Rows {
		if r.UserID != "" || r.AnonIndex != i+1 {
			t.Fatalf("ligne bleue %d : %+v, attendu anonyme numero %d", i, r, i+1)
		}
	}
}

func TestMembreMasqueResteAnonyme(t *testing.T) {
	don := report("u1", "DonZeZe", true, line(0, 400, 2))
	cache := report("u2", "Cache", false, line(0, 300, 1))
	sb := BuildScoreboard(don, []store.RocketLeaguePlayer{cache.Line, line(1, 100, 0)},
		[]store.RocketLeagueReport{cache})
	for _, r := range sb.Teams[0].Rows {
		if r.Username == "Cache" || r.UserID == "u2" {
			t.Fatal("un membre masque ne doit jamais etre nomme")
		}
	}
	if sb.Teams[0].Rows[1].AnonIndex != 1 {
		t.Fatalf("le membre masque doit etre Coequipier 1 : %+v", sb.Teams[0].Rows[1])
	}
}

func TestCorrespondanceApprochee(t *testing.T) {
	// La ligne vue par DonZeZe differe d'une passe de celle rapportee par
	// Osiris (instantanes differents) : on prend le score le plus proche.
	don := report("u1", "DonZeZe", true, line(0, 400, 2))
	osi := report("u2", "Osiris", true, line(0, 310, 1))
	sb := BuildScoreboard(don, []store.RocketLeaguePlayer{line(0, 90, 0), line(0, 300, 1), line(1, 100, 0)},
		[]store.RocketLeagueReport{osi})
	if sb.Teams[0].Rows[1].Username != "Osiris" {
		t.Fatalf("Osiris aurait du prendre la ligne a 300 : %+v", sb.Teams[0].Rows)
	}
	if sb.Teams[0].Rows[2].UserID != "" {
		t.Fatalf("la ligne a 90 doit rester anonyme : %+v", sb.Teams[0].Rows[2])
	}
}

func TestJoueurPartiEtEgalite(t *testing.T) {
	don := report("u1", "DonZeZe", true, line(0, 200, 1))
	don.TeamBlueScore, don.TeamOrangeScore = 1, 1
	parti := line(1, 50, 0)
	parti.Left = true
	sb := BuildScoreboard(don, []store.RocketLeaguePlayer{parti}, nil)
	if sb.Teams[0].Winner || sb.Teams[1].Winner {
		t.Fatal("pas de vainqueur sur une egalite")
	}
	for _, team := range sb.Teams {
		for _, r := range team.Rows {
			if r.MVP {
				t.Fatal("pas de MVP sur une egalite")
			}
		}
	}
	if !sb.Teams[1].Rows[0].Player.Left {
		t.Fatal("le joueur parti doit garder son drapeau")
	}
}

func TestMatchEntreMembres(t *testing.T) {
	don := report("u1", "DonZeZe", true, line(0, 400, 2))
	adv := report("u2", "Kaeloo", true, line(1, 350, 1))
	sb := BuildScoreboard(don, []store.RocketLeaguePlayer{adv.Line}, []store.RocketLeagueReport{adv})
	if sb.Teams[1].Rows[0].Username != "Kaeloo" {
		t.Fatalf("un membre adverse doit etre nomme aussi : %+v", sb.Teams[1].Rows)
	}
}
