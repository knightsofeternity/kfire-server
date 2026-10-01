package store

import (
	"fmt"
	"testing"
	"time"
)

// A /played never goes back in time, and the edition's total is the sum of
// the characters.
func TestWowPlayedIsMonotonicAndSums(t *testing.T) {
	st, ctx := openRatingDB(t)
	tag := fmt.Sprint(time.Now().UnixNano())
	var orgID, userID, gameID string
	if err := st.pool.QueryRow(ctx, `SELECT id FROM orgs LIMIT 1`).Scan(&orgID); err != nil {
		st.pool.QueryRow(ctx, `INSERT INTO orgs (name, slug) VALUES ('t', 't') RETURNING id`).Scan(&orgID)
	}
	st.pool.QueryRow(ctx, `INSERT INTO users (org_id, username, email, password_hash) VALUES ($1, $2, $3, 'x') RETURNING id`,
		orgID, "w"+tag, "w"+tag+"@t.test").Scan(&userID)
	st.pool.QueryRow(ctx, `INSERT INTO games (name, slug, executable_names, platform) VALUES ($1, $2, '{}', 'pc') RETURNING id`,
		"WoW "+tag, "wow-"+tag).Scan(&gameID)
	t.Cleanup(func() {
		st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		st.pool.Exec(ctx, `DELETE FROM games WHERE id = $1`, gameID)
	})
	at := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	ch := func(name string, played int64, when time.Time) WowPlayed {
		return WowPlayed{Region: "eu", RealmNorm: "Chogall", Realm: "Cho'gall", Name: name, PlayedSeconds: played, RecordedAt: when}
	}
	if err := st.UpsertWowPlayed(ctx, userID, gameID, []WowPlayed{ch("Ouranos", 1000, at), ch("Alt", 500, at)}); err != nil {
		t.Fatal(err)
	}
	// An older file (another account folder, a stale copy) must not lower it.
	st.UpsertWowPlayed(ctx, userID, gameID, []WowPlayed{ch("Ouranos", 10, at.Add(-time.Hour))})
	// A newer record does replace it.
	st.UpsertWowPlayed(ctx, userID, gameID, []WowPlayed{ch("Alt", 800, at.Add(time.Hour))})
	total, err := st.WowPlayedTotal(ctx, userID, gameID)
	if err != nil || total != 1800 {
		t.Fatalf("total = %d (err %v), want 1000 + 800", total, err)
	}
	list, _ := st.WowPlayedForUserGame(ctx, userID, gameID)
	if len(list) != 2 || list[0].Name != "Ouranos" {
		t.Fatalf("list = %+v, want most played first", list)
	}
	if err := st.UpsertExternalPlaytime(ctx, userID, "wow_addon", gameID, total); err != nil {
		t.Fatalf("wow_addon must be an accepted provider: %v", err)
	}
}
