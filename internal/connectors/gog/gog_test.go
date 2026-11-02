package gog

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fake(t *testing.T, h http.HandlerFunc) *Connector {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("cid", "secret")
	c.AuthBase, c.EmbedBase, c.LibraryBase, c.GameplayBase, c.ProductsBase, c.ContentBase = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
	return c
}

func TestEnabledAndLoginURL(t *testing.T) {
	if New("", "").Enabled() || New("a", "").Enabled() {
		t.Fatal("both credentials are needed")
	}
	u := New("cid", "the-secret").LoginURL()
	if !strings.HasPrefix(u, "https://auth.gog.com/auth?") || !strings.Contains(u, "client_id=cid") ||
		!strings.Contains(u, "on_login_success") || strings.Contains(u, "the-secret") {
		t.Fatalf("login url = %s", u)
	}
}

func TestParseCode(t *testing.T) {
	code := "YjFvfcjsC14s4yULQGEMvWVVZ1qK9P8-MI3L3Hwod_KzBz6Q"
	cases := map[string]bool{
		"https://embed.gog.com/on_login_success?origin=client&code=" + code: true,
		"  " + code + "\n": true,
		"https://embed.gog.com/on_login_success?origin=client": false,
		"hello": false,
		"":      false,
	}
	for in, ok := range cases {
		got, found := ParseCode(in)
		if found != ok || (ok && got != code) {
			t.Errorf("ParseCode(%q) = %q, %v", in, got, found)
		}
	}
}

func TestExchangeAndRefresh(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/token" || q.Get("client_id") != "cid" || q.Get("client_secret") != "secret" {
			t.Errorf("token call = %s", r.URL)
		}
		switch q.Get("grant_type") {
		case "authorization_code":
			if q.Get("code") != "good" || q.Get("redirect_uri") != Redirect {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"invalid_grant","error_description":"code expired"}`)
				return
			}
		case "refresh_token":
			if q.Get("refresh_token") == "bad-client" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"invalid_client"}`)
				return
			}
			if q.Get("refresh_token") != "R1" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"invalid_grant"}`)
				return
			}
		}
		fmt.Fprint(w, `{"expires_in":3600,"scope":"","token_type":"bearer","access_token":"A","user_id":"541","refresh_token":"R1","session_id":"s"}`)
	})
	tok, err := c.Exchange(context.Background(), "good")
	if err != nil || tok.Access != "A" || tok.Refresh != "R1" || tok.UserID != "541" {
		t.Fatalf("exchange = %+v, %v", tok, err)
	}
	if _, err := c.Exchange(context.Background(), "used"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("bad code: %v", err)
	}
	if _, err := c.Refresh(context.Background(), "R1"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := c.Refresh(context.Background(), "revoked"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("revoked: %v", err)
	}
	if _, err := c.Refresh(context.Background(), "bad-client"); !errors.Is(err, ErrClientRejected) {
		t.Fatalf("bad client: %v", err)
	}
}

func TestLibraryProductsAndPlaytime(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products" && r.Header.Get("Authorization") != "Bearer A" {
			t.Errorf("%s: auth = %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		switch {
		case r.URL.Path == "/userData.json":
			fmt.Fprint(w, `{"username":"OuranosKE","galaxyUserId":"541"}`)
		case r.URL.Path == "/users/541/releases" && r.URL.Query().Get("page_token") == "":
			// Galaxy also lists games imported from other launchers.
			fmt.Fprint(w, `{"total_count":3,"limit":2,"items":[{"platform_id":"steam","external_id":"805550"},{"platform_id":"gog","external_id":"1207658924"}],"next_page_token":"P2"}`)
		case r.URL.Path == "/users/541/releases" && r.URL.Query().Get("page_token") == "P2":
			fmt.Fprint(w, `{"total_count":3,"limit":2,"items":[{"platform_id":"gog","external_id":"1495134320"}]}`)
		case r.URL.Path == "/products":
			if r.URL.Query().Get("ids") != "1207658924,1495134320" {
				t.Errorf("ids = %q", r.URL.Query().Get("ids"))
			}
			fmt.Fprint(w, `[{"id":1207658924,"title":"The Witcher: Enhanced Edition","game_type":"game","images":{"logo2x":"//images.gog.com/w.jpg","icon":"//images.gog.com/i.png"}},
				{"id":1495134320,"title":"The Witcher 3: Wild Hunt","game_type":"game","images":{"icon":"//images.gog.com/w3.png"}}]`)
		case r.URL.Path == "/games/1207658924/users/541/sessions":
			fmt.Fprint(w, `{"time_sum":754}`)
		case r.URL.Path == "/games/1495134320/users/541/sessions":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected %s", r.URL)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	ctx := context.Background()
	if name, err := c.Username(ctx, "A"); err != nil || name != "OuranosKE" {
		t.Fatalf("username = %q, %v", name, err)
	}
	ids, err := c.OwnedGOGProducts(ctx, "A", "541")
	if err != nil || strings.Join(ids, ",") != "1207658924,1495134320" {
		t.Fatalf("owned = %v, %v", ids, err)
	}
	prods, err := c.Products(ctx, ids)
	if err != nil || len(prods) != 2 || prods[0].Image != "https://images.gog.com/w.jpg" || prods[1].Image != "https://images.gog.com/w3.png" || prods[0].GameType != "game" {
		t.Fatalf("products = %+v, %v", prods, err)
	}
	if m, err := c.PlayedMinutes(ctx, "A", "541", "1207658924"); err != nil || m != 754 {
		t.Fatalf("played = %d, %v", m, err)
	}
	if m, err := c.PlayedMinutes(ctx, "A", "541", "1495134320"); err != nil || m != 0 {
		t.Fatalf("never played = %d, %v", m, err)
	}
}

func TestExpiredAccessToken(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	if _, err := c.OwnedGOGProducts(context.Background(), "old", "541"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("401: %v", err)
	}
}

func TestGameClientAndAchievements(t *testing.T) {
	var manifest bytes.Buffer
	zw := zlib.NewWriter(&manifest)
	fmt.Fprint(zw, `{"baseProductId":"1207658924","clientId":"gc1","clientSecret":"gs1"}`)
	zw.Close()
	var base string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/1207658924/os/windows/builds":
			// Only a macOS build carries Galaxy here.
			fmt.Fprint(w, `{"total_count":0,"items":[]}`)
		case "/products/1207658924/os/osx/builds":
			fmt.Fprintf(w, `{"items":[{"generation":2,"link":"%s/meta/m1"}]}`, base)
		case "/meta/m1":
			w.Write(manifest.Bytes())
		case "/products/42/os/windows/builds", "/products/42/os/osx/builds":
			w.WriteHeader(http.StatusNotFound)
		case "/token":
			q := r.URL.Query()
			if q.Get("client_id") != "gc1" || q.Get("client_secret") != "gs1" || q.Get("without_new_session") != "1" {
				t.Errorf("game token = %s", r.URL)
			}
			fmt.Fprint(w, `{"access_token":"GA","refresh_token":"R2","user_id":"541"}`)
		case "/clients/gc1/users/541/achievements":
			if r.Header.Get("Authorization") != "Bearer GA" {
				t.Errorf("auth = %q", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("page_token") == "" {
				fmt.Fprint(w, `{"total_count":3,"page_token":"P2","items":[
					{"achievement_id":"a1","name":"One","image_url_unlocked":"//images.gog.com/a1.png","date_unlocked":"2025-03-04T10:20:30+0000"},
					{"achievement_id":"a2","name":"Two","date_unlocked":null}]}`)
				return
			}
			fmt.Fprint(w, `{"total_count":3,"page_token":"P3","items":[
				{"achievement_id":"a3","name":"Three","date_unlocked":"2025-05-06T00:00:00+00:00"}]}`)
		default:
			t.Errorf("unexpected %s", r.URL)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	base = c.ContentBase
	ctx := context.Background()
	id, secret, ok, err := c.GameClient(ctx, "1207658924")
	if err != nil || !ok || id != "gc1" || secret != "gs1" {
		t.Fatalf("game client = %q %q %v %v", id, secret, ok, err)
	}
	if _, _, ok, err := c.GameClient(ctx, "42"); err != nil || ok {
		t.Fatalf("no build: %v %v", ok, err)
	}
	access, err := c.GameToken(ctx, "gc1", "gs1", "R1")
	if err != nil || access != "GA" {
		t.Fatalf("game token = %q, %v", access, err)
	}
	achs, err := c.Achievements(ctx, access, "gc1", "541")
	if err != nil || len(achs) != 2 || achs[0].ID != "a1" || achs[0].Icon != "https://images.gog.com/a1.png" || achs[1].ID != "a3" ||
		achs[1].UnlockedAt.Month() != 5 {
		t.Fatalf("achievements = %+v, %v", achs, err)
	}
}
