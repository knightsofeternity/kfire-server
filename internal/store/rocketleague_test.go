package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// Reproduit le bug remonté le 2026-09-18 : le jeu signale la fin d'un match
// deux fois (MatchEnded puis MatchDestroyed), le client rapporte les deux, et
// comme played_at est posé au moment du rapport les deux passent l'unicité.
//
// Ce test a besoin d'une vraie base, parce que le garde EST du SQL : le tester
// autrement ne testerait que du Go autour du seul endroit qui peut se tromper.
// Il se saute sans TEST_DATABASE_URL, TEST_USER_ID et TEST_GAME_ID, qui doivent
// désigner un membre et un jeu existants.
func TestInsertRocketLeagueMatchRefuseUnDoublonTardif(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL absent")
	}
	st, err := New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	userID := os.Getenv("TEST_USER_ID")
	gameID := os.Getenv("TEST_GAME_ID")
	if userID == "" || gameID == "" {
		t.Skip("TEST_USER_ID ou TEST_GAME_ID absent")
	}

	base := RocketLeagueMatch{
		UserID: userID, GameID: gameID, TeamSize: 2, PlayerTeam: 1,
		TeamBlueScore: 1, TeamOrangeScore: 2, Result: "loss",
		Goals: 1, Assists: 0, Saves: 2, Shots: 3, Score: 210, Demos: 0,
		DurationSeconds: 372, PlayedAt: time.Now().UTC(),
	}
	if err := st.InsertRocketLeagueMatch(ctx, base); err != nil {
		t.Fatalf("premier: %v", err)
	}
	// Le second rapport : même match, 20 s plus tard, durée écrasée à presque rien.
	dup := base
	dup.PlayedAt = base.PlayedAt.Add(20 * time.Second)
	dup.DurationSeconds = 3
	if err := st.InsertRocketLeagueMatch(ctx, dup); err != nil {
		t.Fatalf("doublon: %v", err)
	}

	var n int
	if err := st.pool.QueryRow(ctx,
		`SELECT count(*) FROM rocket_league_matches WHERE user_id=$1 AND played_at >= $2`,
		userID, base.PlayedAt).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("%d lignes, attendu 1 : le doublon est passé", n)
	}

	// Et un VRAI second match, différent, doit passer.
	other := base
	other.PlayedAt = base.PlayedAt.Add(40 * time.Second)
	other.Goals = 2
	if err := st.InsertRocketLeagueMatch(ctx, other); err != nil {
		t.Fatalf("autre match: %v", err)
	}
	if err := st.pool.QueryRow(ctx,
		`SELECT count(*) FROM rocket_league_matches WHERE user_id=$1 AND played_at >= $2`,
		userID, base.PlayedAt).Scan(&n); err != nil {
		t.Fatalf("count 2: %v", err)
	}
	if n != 2 {
		t.Fatalf("%d lignes, attendu 2 : un match différent a été refusé à tort", n)
	}
	_, _ = st.pool.Exec(ctx, `DELETE FROM rocket_league_matches WHERE user_id=$1 AND played_at >= $2`, userID, base.PlayedAt)
}

// Un match ecarte par la garde anti-doublon ne doit laisser aucun joueur
// orphelin : l'insertion du match et de ses joueurs est atomique.
func TestInsertRocketLeagueMatchAvecJoueurs(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	userID := os.Getenv("TEST_USER_ID")
	gameID := os.Getenv("TEST_GAME_ID")
	if dsn == "" || userID == "" || gameID == "" {
		t.Skip("TEST_DATABASE_URL, TEST_USER_ID ou TEST_GAME_ID absent")
	}
	ctx := context.Background()
	st, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	key := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	m := RocketLeagueMatch{
		UserID: userID, GameID: gameID, TeamSize: 1, PlayerTeam: 0,
		TeamBlueScore: 2, TeamOrangeScore: 1, Result: "win",
		Goals: 2, Score: 400, DurationSeconds: 300, PlayedAt: time.Now().UTC(),
		MatchKey: &key,
		Others:   []RocketLeaguePlayer{{Team: 1, Score: 150, Goals: 1}},
	}
	if err := st.InsertRocketLeagueMatch(ctx, m); err != nil {
		t.Fatalf("insert: %v", err)
	}
	defer st.pool.Exec(ctx, `DELETE FROM rocket_league_matches WHERE match_key = $1`, key)

	// Doublon a 30 s : ecarte, et ses joueurs avec lui.
	dup := m
	dup.PlayedAt = m.PlayedAt.Add(30 * time.Second)
	if err := st.InsertRocketLeagueMatch(ctx, dup); err != nil {
		t.Fatalf("insert doublon: %v", err)
	}
	var matches, players int
	st.pool.QueryRow(ctx, `SELECT count(*) FROM rocket_league_matches WHERE match_key = $1`, key).Scan(&matches)
	st.pool.QueryRow(ctx, `SELECT count(*) FROM rocket_league_match_players p
		JOIN rocket_league_matches m ON m.id = p.match_id WHERE m.match_key = $1`, key).Scan(&players)
	if matches != 1 || players != 1 {
		t.Fatalf("%d matchs et %d joueurs, attendu 1 et 1", matches, players)
	}
}
