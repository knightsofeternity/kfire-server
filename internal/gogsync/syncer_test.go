package gogsync

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/knightsofeternity/kfire-server/internal/connectors/gog"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func TestIsGame(t *testing.T) {
	for typ, want := range map[string]bool{"game": true, "dlc": false, "pack": false, "": false} {
		if IsGame(typ) != want {
			t.Errorf("IsGame(%q) = %v", typ, !want)
		}
	}
}

// TestLinkAndSync runs the whole flow against a fake GOG and a real database:
// only GOG games count (not Steam entries Galaxy imported, not DLC), Galaxy's
// minutes become seconds, a refused refresh token asks the member to relink.
func TestLinkAndSync(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL absent")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(ctx) })

	tag := fmt.Sprint(time.Now().UnixNano())
	galaxyID, game, dlc := "g"+tag, "1"+tag[len(tag)-9:], "2"+tag[len(tag)-9:]
	refreshOK := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.URL.Query().Get("grant_type") == "refresh_token" && !refreshOK {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"invalid_grant"}`)
				return
			}
			fmt.Fprintf(w, `{"access_token":"A","refresh_token":"R","user_id":%q}`, galaxyID)
		case "/userData.json":
			fmt.Fprint(w, `{"username":"OuranosKE"}`)
		case "/users/" + galaxyID + "/releases":
			fmt.Fprintf(w, `{"items":[{"platform_id":"steam","external_id":"805550"},{"platform_id":"gog","external_id":%q},{"platform_id":"gog","external_id":%q}]}`, game, dlc)
		case "/products":
			fmt.Fprintf(w, `[{"id":%s,"title":"Gogtest Quest %s - Game of the Year Edition","game_type":"game","images":{"logo2x":"//img/q.jpg"}},
				{"id":%s,"title":"Gogtest Quest Soundtrack","game_type":"dlc"}]`, game, tag, dlc)
		case "/games/" + game + "/users/" + galaxyID + "/sessions":
			fmt.Fprint(w, `{"time_sum":90}`)
		default:
			t.Errorf("unexpected call %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	conn := gog.New("cid", "secret")
	conn.AuthBase, conn.EmbedBase, conn.LibraryBase, conn.GameplayBase, conn.ProductsBase = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
	key := make([]byte, 32)
	rand.Read(key)
	cipher, err := crypto.New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, conn, cipher)

	var orgID, userID string
	if err := db.QueryRow(ctx, `SELECT id FROM orgs LIMIT 1`).Scan(&orgID); err != nil {
		db.QueryRow(ctx, `INSERT INTO orgs (name, slug) VALUES ('t', 't') RETURNING id`).Scan(&orgID)
	}
	db.QueryRow(ctx, `INSERT INTO users (org_id, username, email, password_hash) VALUES ($1, $2, $3, 'x') RETURNING id`,
		orgID, "g"+tag, "g"+tag+"@t.test").Scan(&userID)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		db.Exec(ctx, `DELETE FROM gog_titles WHERE product_id IN ($1, $2)`, game, dlc)
		db.Exec(ctx, `DELETE FROM games WHERE slug LIKE $1`, "gogtest-quest-"+tag+"%")
	})

	if _, err := s.Link(ctx, userID, "no code here"); !errors.Is(err, ErrNoCode) {
		t.Fatalf("paste without code: %v", err)
	}
	name, err := s.Link(ctx, userID, "https://embed.gog.com/on_login_success?origin=client&code=abcdefghijklmnopqrstuvwxyz")
	if err != nil || name != "OuranosKE" {
		t.Fatalf("link = %q, %v", name, err)
	}
	if err := s.SyncUser(ctx, userID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	lib, err := st.OwnedGames(ctx, userID)
	if err != nil || len(lib) != 1 || lib[0].Source != "gog" || lib[0].Game.Name != "Gogtest Quest "+tag {
		t.Fatalf("library = %+v, %v", lib, err)
	}
	var secs int64
	if err := db.QueryRow(ctx, `SELECT total_seconds FROM external_playtime WHERE user_id = $1 AND provider = 'gog'`, userID).Scan(&secs); err != nil || secs != 90*60 {
		t.Fatalf("seconds = %d, %v", secs, err)
	}

	refreshOK = false
	if err := s.SyncUser(ctx, userID); !errors.Is(err, ErrNeedsRelink) {
		t.Fatalf("refused token: %v", err)
	}
	if a, _ := st.GogAccountFor(ctx, userID); a.Status != "needs_relink" {
		t.Fatalf("status = %q", a.Status)
	}
}
