package store

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestEpicLinkTokenTitlesAndLibrary(t *testing.T) {
	st, ctx := openRatingDB(t)
	tag := fmt.Sprint(time.Now().UnixNano())
	var orgID, userID string
	if err := st.pool.QueryRow(ctx, `SELECT id FROM orgs LIMIT 1`).Scan(&orgID); err != nil {
		st.pool.QueryRow(ctx, `INSERT INTO orgs (name, slug) VALUES ('t', 't') RETURNING id`).Scan(&orgID)
	}
	st.pool.QueryRow(ctx, `INSERT INTO users (org_id, username, email, password_hash) VALUES ($1, $2, $3, 'x') RETURNING id`,
		orgID, "e"+tag, "e"+tag+"@t.test").Scan(&userID)
	t.Cleanup(func() { st.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	exp := time.Now().Add(365 * 24 * time.Hour).UTC().Truncate(time.Second)
	if err := st.SaveEpicLink(ctx, userID, "acc"+tag, "OuranosKE", []byte("sealed-1"), exp); err != nil {
		t.Fatal(err)
	}
	if enc, err := st.EpicRefreshToken(ctx, userID); err != nil || string(enc) != "sealed-1" {
		t.Fatalf("token = %q, %v", enc, err)
	}
	if err := st.UpdateEpicToken(ctx, userID, []byte("sealed-2"), exp); err != nil {
		t.Fatal(err)
	}
	if enc, _ := st.EpicRefreshToken(ctx, userID); string(enc) != "sealed-2" {
		t.Fatalf("token after rotation = %q", enc)
	}
	ids, err := st.EpicLinkedOK(ctx)
	if err != nil || !contains(ids, userID) {
		t.Fatalf("linked ok = %v, %v", ids, err)
	}
	if err := st.MarkEpicNeedsRelink(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if a, _ := st.EpicAccountFor(ctx, userID); a.Status != "needs_relink" {
		t.Fatalf("status = %q", a.Status)
	}
	if ids, _ := st.EpicLinkedOK(ctx); contains(ids, userID) {
		t.Fatal("a dead link must not be synced")
	}

	// Title cache and game upsert: a new Epic-only game is created on PC.
	g, err := st.UpsertEpicGame(ctx, "Jotunnslayer Hordes of Hel "+tag, "https://img/x.jpg")
	if err != nil || g.ID == "" {
		t.Fatalf("game = %+v, %v", g, err)
	}
	t.Cleanup(func() { st.pool.Exec(ctx, `DELETE FROM games WHERE id = $1`, g.ID) })
	again, _ := st.UpsertEpicGame(ctx, "Jotunnslayer Hordes of Hel "+tag, "")
	if again.ID != g.ID {
		t.Fatal("the same name must resolve to the same game")
	}
	title := EpicTitle{Namespace: "ns" + tag, CatalogItemID: "cid", AppName: "app" + tag,
		Title: "Jotunnslayer", IsGame: true, ImageURL: "https://img/x.jpg", GameID: &g.ID}
	if err := st.SaveEpicTitle(ctx, title); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.pool.Exec(ctx, `DELETE FROM epic_titles WHERE namespace = $1`, title.Namespace) })
	got, ok, err := st.EpicTitleByKey(ctx, title.Namespace, "cid")
	if err != nil || !ok || got.GameID == nil || *got.GameID != g.ID || !got.IsGame {
		t.Fatalf("title = %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := st.EpicTitleByKey(ctx, "nope", "nope"); ok {
		t.Fatal("unknown title must be absent")
	}

	// Playtime with provider epic is accepted and shows in the library as epic.
	if err := st.UpsertExternalPlaytime(ctx, userID, "epic", g.ID, 0); err != nil {
		t.Fatalf("provider epic: %v", err)
	}
	lib, err := st.OwnedGames(ctx, userID)
	if err != nil || len(lib) != 1 || lib[0].Source != "epic" {
		t.Fatalf("library = %+v, %v", lib, err)
	}

	if err := st.DeleteEpicLink(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EpicAccountFor(ctx, userID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after unlink: %v", err)
	}
	if err := st.DeleteEpicLink(ctx, userID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second unlink: %v, want ErrNotFound", err)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
