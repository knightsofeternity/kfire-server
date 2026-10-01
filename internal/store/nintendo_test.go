package store

import (
	"fmt"
	"testing"
	"time"
)

// A Switch game joins the PC game of the same name, keeps its mapping, and an
// unknown one becomes a single nintendo game with its cover.
func TestNintendoGamesJoinTheCatalog(t *testing.T) {
	st, ctx := openRatingDB(t)
	tag := fmt.Sprint(time.Now().UnixNano())
	pcName := "Minecraft Test " + tag
	var pcID string
	if err := st.pool.QueryRow(ctx, `INSERT INTO games (name, slug, executable_names, platform)
		VALUES ($1, $2, '{}', 'pc') RETURNING id`, pcName, steamSlug(pcName)).Scan(&pcID); err != nil {
		t.Fatal(err)
	}
	newName := "Mario Test " + tag
	title := fmt.Sprintf("0100%012x", time.Now().UnixNano()&0xffffffffffff)
	other := fmt.Sprintf("0400%012x", time.Now().UnixNano()&0xffffffffffff)
	t.Cleanup(func() {
		st.pool.Exec(ctx, `DELETE FROM games WHERE slug IN ($1, $2)`, steamSlug(pcName), steamSlug(newName))
	})

	g, err := st.UpsertNintendoGame(ctx, title, pcName, "https://img/cover.jpg")
	if err != nil || g.ID != pcID {
		t.Fatalf("joined %v (err %v), want the PC game", g.ID, err)
	}
	if g.IconURL == nil || *g.IconURL != "https://img/cover.jpg" {
		t.Fatal("a PC game without an icon takes the eShop cover")
	}
	again, err := st.UpsertNintendoGame(ctx, title, "renamed on the eShop", "")
	if err != nil || again.ID != pcID {
		t.Fatalf("title id must lead back to the same game: %v %v", again.ID, err)
	}
	fresh, err := st.UpsertNintendoGame(ctx, other, newName, "https://img/mario.jpg")
	if err != nil || fresh.ID == pcID {
		t.Fatalf("new game: %+v %v", fresh, err)
	}
	var platform string
	st.pool.QueryRow(ctx, `SELECT platform FROM games WHERE id = $1`, fresh.ID).Scan(&platform)
	if platform != "nintendo" {
		t.Fatalf("platform = %q", platform)
	}
	byName, err := st.UpsertNintendoGame(ctx, "", newName, "")
	if err != nil || byName.ID != fresh.ID {
		t.Fatal("a game without a shop link is found by name")
	}
}

func TestNintendoBotAndFriendCode(t *testing.T) {
	st, ctx := openRatingDB(t)
	st.DeleteNintendoBotForTests(ctx)
	t.Cleanup(func() { st.DeleteNintendoBotForTests(ctx) })
	if _, err := st.GetNintendoBot(ctx); err != ErrNotFound {
		t.Fatalf("no bot yet: %v", err)
	}
	if err := st.StartNintendoLogin(ctx, "state-1", []byte("sealed-v")); err != nil {
		t.Fatal(err)
	}
	b, _ := st.GetNintendoBot(ctx)
	if b.SessionEnc != nil || b.LoginState == nil || *b.LoginState != "state-1" {
		t.Fatalf("pending login: %+v", b)
	}
	if err := st.SetNintendoSession(ctx, []byte("sealed-s"), "KFIREKE", "nsa"); err != nil {
		t.Fatal(err)
	}
	b, _ = st.GetNintendoBot(ctx)
	if b.Status != "ok" || b.LoginState != nil || *b.Nickname != "KFIREKE" || b.LastOKAt == nil {
		t.Fatalf("after login: %+v", b)
	}
	st.MarkNintendoNeedsLogin(ctx, "revoked")
	b, _ = st.GetNintendoBot(ctx)
	if b.Status != "needs_login" || b.LastError == nil {
		t.Fatalf("after revocation: %+v", b)
	}
}
