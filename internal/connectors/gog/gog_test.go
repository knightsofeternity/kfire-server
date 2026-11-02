package gog

import (
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
	c.AuthBase, c.EmbedBase, c.LibraryBase, c.GameplayBase, c.ProductsBase = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
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
