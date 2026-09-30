package store

import (
	"fmt"
	"testing"
	"time"
)

// A PlayStation concept must join the PC game of the same name, every title id
// of the concept must lead back to it, and an unknown title must become a new
// psn game only once. Needs TEST_DATABASE_URL (see openRatingDB).
func TestPsnGamesJoinTheCatalog(t *testing.T) {
	st, ctx := openRatingDB(t)
	tag := fmt.Sprint(time.Now().UnixNano())
	pcName := "Diablo Test " + tag
	pcSlug := psnSlug(pcName)
	var pcID string
	if err := st.pool.QueryRow(ctx, `INSERT INTO games (name, slug, executable_names, platform)
		VALUES ($1, $2, '{}', 'pc') RETURNING id`, pcName, pcSlug).Scan(&pcID); err != nil {
		t.Fatal(err)
	}
	concept := "c" + tag
	ps5, ps4 := "PPSA"+tag, "CUSA"+tag
	t.Cleanup(func() {
		st.pool.Exec(ctx, `DELETE FROM games WHERE slug IN ($1, $2)`, pcSlug, psnSlug("Brand New "+tag))
	})

	g, err := st.UpsertPsnGame(ctx, concept, []string{ps5, ps4}, pcName, "")
	if err != nil || g.ID != pcID {
		t.Fatalf("concept joined %v (err %v), want the PC game %s", g.ID, err, pcID)
	}
	again, err := st.UpsertPsnGame(ctx, concept, []string{ps5}, "renamed on the store", "")
	if err != nil || again.ID != pcID {
		t.Fatalf("second import went to %v (err %v)", again.ID, err)
	}
	viaTitle, err := st.PsnGameForTitle(ctx, ps4, "whatever")
	if err != nil || viaTitle.ID != pcID {
		t.Fatalf("PS4 title id led to %v (err %v)", viaTitle.ID, err)
	}

	fresh, err := st.PsnGameForTitle(ctx, "PPSX"+tag, "Brand New "+tag)
	if err != nil || fresh.Slug != psnSlug("Brand New "+tag) {
		t.Fatalf("new title: %+v err %v", fresh, err)
	}
	same, err := st.PsnGameForTitle(ctx, "PPSX"+tag, "Brand New "+tag)
	if err != nil || same.ID != fresh.ID {
		t.Fatalf("second presence created %v, want %v", same.ID, fresh.ID)
	}
}

// The bot's lifecycle: a new NPSSO clears old tokens, a refusal is recorded,
// and a fresh NPSSO brings the bot back.
func TestPsnBotLifecycle(t *testing.T) {
	st, ctx := openRatingDB(t)
	t.Cleanup(func() { st.pool.Exec(ctx, `DELETE FROM psn_bot`) })
	if _, err := st.GetPsnBot(ctx); err != ErrNotFound {
		st.pool.Exec(ctx, `DELETE FROM psn_bot`)
	}
	if err := st.SetPsnNPSSO(ctx, []byte("sealed-1")); err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Hour)
	if err := st.SavePsnTokens(ctx, []byte("a"), exp, []byte("r"), exp, "kfirebot", "acc"); err != nil {
		t.Fatal(err)
	}
	b, err := st.GetPsnBot(ctx)
	if err != nil || b.Status != "ok" || b.OnlineID == nil || *b.OnlineID != "kfirebot" || b.AccessTokenEnc == nil || b.LastOKAt == nil {
		t.Fatalf("after tokens: %+v err %v", b, err)
	}
	if err := st.MarkPsnNeedsNPSSO(ctx, "refused"); err != nil {
		t.Fatal(err)
	}
	b, _ = st.GetPsnBot(ctx)
	if b.Status != "needs_npsso" || b.AccessTokenEnc != nil || b.LastError == nil {
		t.Fatalf("after refusal: %+v", b)
	}
	if err := st.SetPsnNPSSO(ctx, []byte("sealed-2")); err != nil {
		t.Fatal(err)
	}
	b, _ = st.GetPsnBot(ctx)
	if b.Status != "ok" || b.LastError != nil || string(b.NPSSOEnc) != "sealed-2" || *b.OnlineID != "kfirebot" {
		t.Fatalf("after new NPSSO: %+v", b)
	}
}
