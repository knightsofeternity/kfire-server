package store

import (
	"fmt"
	"testing"
	"time"
)

// A member playing on PC and on a console at once has two open sessions: both
// are returned, newest first, and a hidden game is never one of them.
func TestOpenSessionsOfOneMember(t *testing.T) {
	st, ctx := openRatingDB(t)
	tag := fmt.Sprint(time.Now().UnixNano())
	var orgID, userID string
	if err := st.pool.QueryRow(ctx, `SELECT id FROM orgs LIMIT 1`).Scan(&orgID); err != nil {
		if err := st.pool.QueryRow(ctx, `INSERT INTO orgs (name, slug) VALUES ('t', 't') RETURNING id`).Scan(&orgID); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.pool.QueryRow(ctx, `INSERT INTO users (org_id, username, email, password_hash)
		VALUES ($1, $2, $3, 'x') RETURNING id`, orgID, "u"+tag, tag+"@t.test").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	game := func(name string, hidden bool) string {
		var id string
		if err := st.pool.QueryRow(ctx, `INSERT INTO games (name, slug, executable_names, platform, hidden)
			VALUES ($1, $2, '{}', 'pc', $3) RETURNING id`, name, steamSlug(name), hidden).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	pc, sw, hid := game("PC "+tag, false), game("Switch "+tag, false), game("Hidden "+tag, true)
	t.Cleanup(func() {
		st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		st.pool.Exec(ctx, `DELETE FROM games WHERE id IN ($1, $2, $3)`, pc, sw, hid)
	})
	if _, err := st.StartSession(ctx, userID, pc, "client"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := st.StartSession(ctx, userID, sw, "nintendo_api"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.StartSession(ctx, userID, hid, "client"); err != nil {
		t.Fatal(err)
	}

	open, err := st.OpenSessionsForUser(ctx, userID)
	if err != nil || len(open) != 2 {
		t.Fatalf("open=%d err=%v, want the two visible games", len(open), err)
	}
	if open[0].Game.ID != sw || open[0].Source != "nintendo_api" || open[1].Game.ID != pc {
		t.Fatalf("order: %s then %s, want the Switch game first (newest)", open[0].Game.Name, open[1].Game.Name)
	}
	all, err := st.OpenSessionsByUser(ctx)
	if err != nil || len(all[userID]) != 2 {
		t.Fatalf("by user: %d err=%v", len(all[userID]), err)
	}
}
