package epic

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
	c.AuthBase, c.LibraryBase, c.CatalogBase = srv.URL+"/oauth", srv.URL+"/library", srv.URL+"/catalog"
	return c
}

func TestEnabledAndLoginURL(t *testing.T) {
	if New("", "").Enabled() || New("a", "").Enabled() {
		t.Fatal("both credentials are needed")
	}
	u := New("cid", "the-secret").LoginURL()
	if !strings.HasPrefix(u, "https://www.epicgames.com/id/login?redirectUrl=") || !strings.Contains(u, "clientId%3Dcid") || strings.Contains(u, "the-secret") {
		t.Fatalf("login url = %s", u)
	}
}

func TestExchangeAndRefresh(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != "cid" || pass != "secret" {
			t.Errorf("basic auth = %s:%s", user, pass)
		}
		r.ParseForm()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "good" || r.Form.Get("token_type") != "eg1" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"errorCode":"errors.com.epicgames.account.oauth.authorization_code_not_found"}`)
				return
			}
		case "refresh_token":
			if r.Form.Get("refresh_token") != "R1" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"errorCode":"errors.com.epicgames.account.auth_token.invalid_refresh_token"}`)
				return
			}
		}
		fmt.Fprint(w, `{"access_token":"A","expires_in":129600,"refresh_token":"R2","refresh_expires":31540000,"account_id":"acc","displayName":"OuranosKE"}`)
	})
	tok, err := c.Exchange(context.Background(), "good")
	if err != nil || tok.Access != "A" || tok.Refresh != "R2" || tok.AccountID != "acc" || tok.DisplayName != "OuranosKE" || tok.RefreshExpires.IsZero() {
		t.Fatalf("exchange = %+v, %v", tok, err)
	}
	if _, err := c.Exchange(context.Background(), "used"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("bad code: %v", err)
	}
	if tok, err := c.Refresh(context.Background(), "R1"); err != nil || tok.Refresh != "R2" {
		t.Fatalf("refresh = %+v, %v", tok, err)
	}
	if _, err := c.Refresh(context.Background(), "dead"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("dead refresh: %v", err)
	}
}

func TestClientRejected(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"errorCode":"errors.com.epicgames.account.invalid_client_credentials"}`)
	})
	if _, err := c.Exchange(context.Background(), "x"); !errors.Is(err, ErrClientRejected) {
		t.Fatalf("err = %v, want ErrClientRejected", err)
	}
}

func TestLibraryPagesCatalogAndPlaytime(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "bearer A" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.URL.Path == "/library/items" && r.URL.Query().Get("cursor") == "":
			fmt.Fprint(w, `{"responseMetadata":{"nextCursor":"p2"},"records":[{"appName":"Fortnite","namespace":"fn","catalogItemId":"c1"}]}`)
		case r.URL.Path == "/library/items" && r.URL.Query().Get("cursor") == "p2":
			fmt.Fprint(w, `{"responseMetadata":{},"records":[{"appName":"Sugar","namespace":"rl","catalogItemId":"c2"}]}`)
		case r.URL.Path == "/catalog/namespace/rl/bulk/items":
			if r.URL.Query().Get("id") != "c2" {
				t.Errorf("id = %q", r.URL.Query().Get("id"))
			}
			fmt.Fprint(w, `{"c2":{"title":"Rocket League®","categories":[{"path":"public"},{"path":"games"}],
				"keyImages":[{"type":"Thumbnail","url":"https://t"},{"type":"DieselGameBoxTall","url":"https://tall"}]}}`)
		case r.URL.Path == "/library/playtime/account/acc/all":
			fmt.Fprint(w, `[{"accountId":"acc","artifactId":"Fortnite","totalTime":412468}]`)
		default:
			t.Errorf("unexpected %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	ctx := context.Background()
	items, err := c.Library(ctx, "A")
	if err != nil || len(items) != 2 || items[1].AppName != "Sugar" {
		t.Fatalf("library = %+v, %v", items, err)
	}
	cat, err := c.CatalogItem(ctx, "A", "rl", "c2")
	if err != nil || cat.Title != "Rocket League®" || cat.Image != "https://tall" || len(cat.Categories) != 2 {
		t.Fatalf("catalog = %+v, %v", cat, err)
	}
	play, err := c.Playtime(ctx, "A", "acc")
	if err != nil || len(play) != 1 || play[0].ArtifactID != "Fortnite" || play[0].TotalTime != 412468 {
		t.Fatalf("playtime = %+v, %v", play, err)
	}
}
