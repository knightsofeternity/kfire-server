package gogsync

import (
	"bytes"
	"compress/zlib"
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
// minutes become seconds, unlocked achievements come through the game's own
// Galaxy client, a refused refresh token asks the member to relink.
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
	var manifest bytes.Buffer
	zw := zlib.NewWriter(&manifest)
	fmt.Fprint(zw, `{"clientId":"gc1","clientSecret":"gs1"}`)
	zw.Close()
	metaCalls := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if q := r.URL.Query(); q.Get("client_id") == "gc1" {
				if q.Get("client_secret") != "gs1" || q.Get("refresh_token") != "R" || q.Get("without_new_session") != "1" {
					t.Errorf("game token call = %s", r.URL)
				}
				fmt.Fprintf(w, `{"access_token":"GA","refresh_token":"other","user_id":%q}`, galaxyID)
				return
			}
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
		case "/products/" + game + "/os/windows/builds":
			fmt.Fprintf(w, `{"items":[{"generation":2,"link":"%s/meta/m1"}]}`, srv.URL)
		case "/meta/m1":
			metaCalls++
			w.Write(manifest.Bytes())
		case "/clients/gc1/users/" + galaxyID + "/achievements":
			if r.Header.Get("Authorization") != "Bearer GA" {
				t.Errorf("achievements auth = %q", r.Header.Get("Authorization"))
			}
			fmt.Fprint(w, `{"total_count":2,"page_token":"x","items":[
				{"achievement_id":"a1","name":"First Blood","image_url_unlocked":"//img/a1.png","date_unlocked":"2025-03-04T10:20:30+0000"},
				{"achievement_id":"a2","name":"Locked","image_url_unlocked":"//img/a2.png","date_unlocked":null}]}`)
		default:
			t.Errorf("unexpected call %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	conn := gog.New("cid", "secret")
	conn.AuthBase, conn.EmbedBase, conn.LibraryBase, conn.GameplayBase, conn.ProductsBase, conn.ContentBase = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
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
	achs, err := st.ListUserAchievements(ctx, userID, lib[0].Game.ID, 10, 0)
	if err != nil || len(achs) != 1 || achs[0].APIName != "a1" || *achs[0].IconURL != "https://img/a1.png" ||
		!achs[0].UnlockedAt.Equal(time.Date(2025, 3, 4, 10, 20, 30, 0, time.UTC)) {
		t.Fatalf("achievements = %+v, %v", achs, err)
	}
	// A second sync reads the cached client, not the manifest again.
	if err := s.SyncUser(ctx, userID); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if metaCalls != 1 {
		t.Fatalf("manifest read %d times", metaCalls)
	}

	refreshOK = false
	if err := s.SyncUser(ctx, userID); !errors.Is(err, ErrNeedsRelink) {
		t.Fatalf("refused token: %v", err)
	}
	if a, _ := st.GogAccountFor(ctx, userID); a.Status != "needs_relink" {
		t.Fatalf("status = %q", a.Status)
	}
}
