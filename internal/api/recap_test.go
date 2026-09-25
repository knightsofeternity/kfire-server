package api

import (
	"github.com/gofiber/fiber/v2"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

func TestParseRecapWindow(t *testing.T) {
	cas := []struct {
		nom   string
		from  string
		to    string
		valid bool
	}{
		{"soiree normale", "2026-09-17T20:00:00Z", "2026-09-18T02:00:00Z", true},
		{"plage inversee", "2026-09-18T02:00:00Z", "2026-09-17T20:00:00Z", false},
		{"plage de duree nulle", "2026-09-17T20:00:00Z", "2026-09-17T20:00:00Z", false},
		{"sept jours pile", "2026-09-11T20:00:00Z", "2026-09-18T20:00:00Z", true},
		{"sept jours et une seconde", "2026-09-11T20:00:00Z", "2026-09-18T20:00:01Z", false},
		{"date mal formee", "17/09/2026 20:00", "2026-09-18T02:00:00Z", false},
		{"from absent", "", "2026-09-18T02:00:00Z", false},
		{"to absent", "2026-09-17T20:00:00Z", "", false},
		// Un décalage horaire est une saisie normale depuis le navigateur :
		// c'est l'instant qui compte, pas le fuseau qui l'écrit.
		{"avec decalage horaire", "2026-09-17T22:00:00+02:00", "2026-09-18T04:00:00+02:00", true},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			w, msg, err := parseRecapWindow(c.from, c.to)
			if c.valid {
				if err != nil {
					t.Fatalf("refusée alors qu'elle est valide : %s", msg)
				}
				if !w.To.After(w.From) {
					t.Fatalf("bornes incohérentes : %v -> %v", w.From, w.To)
				}
				return
			}
			if err == nil {
				t.Fatal("acceptée alors qu'elle est invalide")
			}
			if msg == "" {
				t.Fatal("refus sans message : la page n'a rien à afficher")
			}
		})
	}
}

// owner et game fabriquent les deux moitiés communes d'un match de test.
func owner(id, name string) store.RecapMatchOwner {
	return store.RecapMatchOwner{UserID: id, Username: name}
}

func game(id, slug, name string) store.RecapGame {
	return store.RecapGame{GameID: id, GameSlug: slug, GameName: name}
}

var (
	rlGame = game("g-rl", "rocket-league", "Rocket League")
	hsGame = game("g-hs", "hearthstone", "Hearthstone")
	debut  = time.Date(2026, 9, 17, 20, 0, 0, 0, time.UTC)
	fin    = time.Date(2026, 9, 18, 2, 0, 0, 0, time.UTC)
)

func rlMatch(o store.RecapMatchOwner, at time.Time, result string, goals int) store.RecapRocketLeagueMatch {
	return store.RecapRocketLeagueMatch{
		RecapMatchOwner: o, RecapGame: rlGame, PlayedAt: at, Result: result,
		Goals: goals, Assists: 1, Saves: 2, Shots: 3, Score: 300, Demos: 1,
		TeamSize: 2, PlayerTeam: 0, DurationSeconds: 300,
	}
}

func hsMatch(o store.RecapMatchOwner, at time.Time, result string, placement *int) store.RecapHearthstoneMatch {
	return store.RecapHearthstoneMatch{
		RecapMatchOwner: o, RecapGame: hsGame, PlayedAt: at,
		Mode: "battlegrounds", Result: result, Placement: placement,
	}
}

func placement(p int) *int { return &p }

func TestAggregateRocketLeague(t *testing.T) {
	don := owner("u1", "DonZeZe")
	kae := owner("u2", "Kaeloo")
	// Un match exactement sur chaque borne de la plage : c'est là que ce genre
	// de code se trompe, et les deux doivent compter puisque la base les a
	// renvoyés (fenêtre semi-ouverte, le second est à to - 1s).
	matches := []store.RecapRocketLeagueMatch{
		rlMatch(don, debut, "win", 2),
		rlMatch(kae, debut.Add(time.Hour), "loss", 0),
		rlMatch(don, debut.Add(2*time.Hour), "draw", 1),
		rlMatch(don, fin.Add(-time.Second), "win", 3),
	}

	blocks := aggregateRocketLeague(matches)
	if len(blocks) != 1 {
		t.Fatalf("%d blocs, attendu 1", len(blocks))
	}
	b := blocks[0]
	if b.GameSlug != "rocket-league" || b.Matches != 4 {
		t.Fatalf("bloc inattendu : %s, %d matchs", b.GameSlug, b.Matches)
	}
	if len(b.Members) != 2 {
		t.Fatalf("%d membres, attendu 2", len(b.Members))
	}
	// Le plus de matchs d'abord.
	first := b.Members[0]
	if first["username"] != "DonZeZe" {
		t.Fatalf("premier membre %v, attendu DonZeZe", first["username"])
	}
	for champ, attendu := range map[string]int{
		"matches": 3, "wins": 2, "losses": 0, "draws": 1,
		"goals": 6, "assists": 3, "saves": 6, "shots": 9, "demos": 3,
		"score": 900, "play_time_seconds": 900, "mvps": 0,
	} {
		if first[champ] != attendu {
			t.Errorf("%s = %v, attendu %d", champ, first[champ], attendu)
		}
	}
	if _, ok := first["avatar_url"]; ok {
		t.Error("avatar_url présent alors qu'il est nul")
	}
	// Rien, nulle part, ne doit dire avec qui un membre a joué.
	for _, interdit := range []string{"teammates", "opponents", "players", "with", "team"} {
		if _, ok := first[interdit]; ok {
			t.Errorf("champ %q : le bilan ne peut pas dire qui a joué avec qui", interdit)
		}
	}
}

func TestAggregateHearthstone(t *testing.T) {
	don := owner("u1", "DonZeZe")
	matches := []store.RecapHearthstoneMatch{
		hsMatch(don, debut, "win", placement(1)),
		hsMatch(don, debut.Add(time.Hour), "loss", placement(5)),
		// Une partie dont le log n'a pas livré la position : elle compte comme
		// partie, mais ne pèse ni sur la moyenne ni sur le top 4.
		hsMatch(don, debut.Add(2*time.Hour), "loss", nil),
	}

	blocks := aggregateHearthstone(matches)
	if len(blocks) != 1 || len(blocks[0].Members) != 1 {
		t.Fatalf("attendu 1 bloc et 1 membre, obtenu %d bloc(s)", len(blocks))
	}
	m := blocks[0].Members[0]
	for champ, attendu := range map[string]int{
		"matches": 3, "wins": 1, "losses": 2, "draws": 0, "ranked": 2, "top4": 1,
	} {
		if m[champ] != attendu {
			t.Errorf("%s = %v, attendu %d", champ, m[champ], attendu)
		}
	}
	if got := m["avg_placement"]; got != 3.0 {
		t.Errorf("avg_placement = %v, attendu 3", got)
	}
}

func TestAggregateHearthstoneSansPlacement(t *testing.T) {
	don := owner("u1", "DonZeZe")
	blocks := aggregateHearthstone([]store.RecapHearthstoneMatch{
		hsMatch(don, debut, "loss", nil),
	})
	// Une moyenne de rien n'est pas une première place.
	if got := blocks[0].Members[0]["avg_placement"]; got != nil {
		t.Errorf("avg_placement = %v, attendu nul", got)
	}
}

func TestAggregateVide(t *testing.T) {
	if b := aggregateRocketLeague(nil); len(b) != 0 {
		t.Errorf("rocket league : %d blocs pour aucun match", len(b))
	}
	if b := aggregateHearthstone(nil); len(b) != 0 {
		t.Errorf("hearthstone : %d blocs pour aucun match", len(b))
	}
	if tl := mergeRecapTimeline("", nil, nil); len(tl) != 0 {
		t.Errorf("chronologie : %d entrées pour aucun match", len(tl))
	}
}

func TestMergeRecapTimeline(t *testing.T) {
	don := owner("u1", "DonZeZe")
	kae := owner("u2", "Kaeloo")
	rl := []store.RecapRocketLeagueMatch{
		rlMatch(don, debut, "win", 1),
		rlMatch(don, debut.Add(2*time.Hour), "loss", 0),
		rlMatch(kae, fin.Add(-time.Second), "win", 2),
	}
	hs := []store.RecapHearthstoneMatch{
		hsMatch(kae, debut.Add(time.Hour), "win", placement(1)),
		// Exactement à la même seconde qu'un match Rocket League : une
		// coïncidence, jamais une partie commune.
		hsMatch(kae, debut.Add(2*time.Hour), "loss", placement(7)),
	}

	tl := mergeRecapTimeline("https://exemple.test", rl, hs)
	if len(tl) != 5 {
		t.Fatalf("%d entrées, attendu 5", len(tl))
	}
	var prev time.Time
	for i, e := range tl {
		at := e["played_at"].(time.Time)
		if at.Before(prev) {
			t.Fatalf("entrée %d hors ordre : %v après %v", i, at, prev)
		}
		prev = at
		if e["game_slug"] == "" {
			t.Fatalf("entrée %d sans game_slug", i)
		}
	}
	if tl[0]["game_slug"] != "rocket-league" || tl[1]["game_slug"] != "hearthstone" {
		t.Fatalf("ordre des jeux inattendu : %v puis %v", tl[0]["game_slug"], tl[1]["game_slug"])
	}
	// La dernière entrée est le match le plus récent, celui posé juste avant
	// la borne de fin.
	if tl[4]["username"] != "Kaeloo" || tl[4]["game_slug"] != "rocket-league" {
		t.Fatalf("dernière entrée inattendue : %v", tl[4])
	}
}

func TestSortRecapBlocks(t *testing.T) {
	blocks := []recapGameBlock{
		{RecapGame: hsGame, Matches: 2},
		{RecapGame: rlGame, Matches: 9},
	}
	sortRecapBlocks(blocks)
	if blocks[0].GameSlug != "rocket-league" {
		t.Fatalf("premier bloc %s, attendu le jeu le plus joué", blocks[0].GameSlug)
	}
}

// TestIconeDuJeuSeulementQuandElleExiste verrouille la seule règle de
// recapGameIcon : un jeu sans icône ne doit produire AUCUN lien.
//
// Émettre le lien dans tous les cas serait plus simple et faux : le cache
// d'images répond 404 pour un jeu sans image source, et la page dessinerait
// une image cassée à côté d'un nom parfaitement valide.
func TestIconeDuJeuSeulementQuandElleExiste(t *testing.T) {
	const base = "https://exemple.test"
	icone := "https://cdn.exemple/app-icons/rl.png"

	avec := game("g-rl", "rocket-league", "Rocket League")
	avec.GameIcon = &icone
	out := fiber.Map{}
	recapGameIcon(out, base, avec)
	if got := out["icon_url"]; got != base+"/img/games/g-rl/icon" {
		t.Fatalf("icon_url = %v", got)
	}

	sans := game("g-x", "un-jeu", "Un jeu")
	out = fiber.Map{}
	recapGameIcon(out, base, sans)
	if _, present := out["icon_url"]; present {
		t.Fatalf("un jeu sans icône ne doit pas porter icon_url : %v", out["icon_url"])
	}
}

func keyed(m store.RecapRocketLeagueMatch, id, key string, team int) store.RecapRocketLeagueMatch {
	m.ID, m.MatchKey, m.PlayerTeam = id, &key, team
	return m
}

func TestTimelineRegroupeUnMatchPartage(t *testing.T) {
	don := owner("u1", "DonZeZe")
	osi := owner("u2", "Osiris")
	kae := owner("u3", "Kaeloo")
	rl := []store.RecapRocketLeagueMatch{
		keyed(rlMatch(osi, debut, "win", 1), "m-osi", "k1", 0),
		keyed(rlMatch(don, debut.Add(20*time.Second), "win", 2), "m-don", "k1", 0),
		rlMatch(kae, debut.Add(time.Hour), "loss", 0),
	}
	tl := mergeRecapTimeline("", rl, nil)
	if len(tl) != 2 {
		t.Fatalf("%d entrees, attendu 2 (un match partage, un match seul)", len(tl))
	}
	g := tl[0]
	members := g["members"].([]fiber.Map)
	if len(members) != 2 || members[0]["username"] != "DonZeZe" || members[1]["username"] != "Osiris" {
		t.Fatalf("membres inattendus : %v", members)
	}
	// Reference : le premier membre par ordre alphabetique.
	if g["id"] != "m-don" || g["has_scoreboard"] != true || g["mixed"] != false {
		t.Fatalf("entree regroupee inattendue : %v", g)
	}
	// Heure : le rapport le plus ancien du groupe.
	if !g["played_at"].(time.Time).Equal(debut) {
		t.Fatalf("heure %v, attendu %v", g["played_at"], debut)
	}
	solo := tl[1]
	if solo["has_scoreboard"] != false || len(solo["members"].([]fiber.Map)) != 1 {
		t.Fatalf("entree seule inattendue : %v", solo)
	}
}

func TestTimelineMatchEntreMembres(t *testing.T) {
	don := owner("u1", "DonZeZe")
	kae := owner("u3", "Kaeloo")
	rl := []store.RecapRocketLeagueMatch{
		keyed(rlMatch(don, debut, "win", 2), "m-don", "k2", 0),
		keyed(rlMatch(kae, debut.Add(5*time.Second), "loss", 0), "m-kae", "k2", 1),
	}
	tl := mergeRecapTimeline("", rl, nil)
	if len(tl) != 1 || tl[0]["mixed"] != true {
		t.Fatalf("attendu une seule entree marquee entre membres : %v", tl)
	}
}
