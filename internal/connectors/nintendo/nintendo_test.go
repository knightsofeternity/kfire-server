package nintendo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeSidecar answers /api/znc/call by Coral path.
func fakeSidecar(t *testing.T, routes map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "na SESSION" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body struct {
			URL       string         `json:"url"`
			Parameter map[string]any `json:"parameter"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		out, ok := routes[body.URL]
		if !ok {
			w.WriteHeader(500)
			fmt.Fprint(w, `{"error":"unknown_error","data":{"status":9402,"errorMessage":"Resource not found."}}`)
			return
		}
		if strings.HasPrefix(out, "HTTP500:") {
			w.WriteHeader(500)
			fmt.Fprint(w, strings.TrimPrefix(out, "HTTP500:"))
			return
		}
		fmt.Fprint(w, out)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

func TestFriendsAndPresence(t *testing.T) {
	c := fakeSidecar(t, map[string]string{"/v4/Friend/List": `{"status":0,"result":{"friends":[
		{"nsaId":"a1","name":"Ouranos","presence":{"state":"ONLINE","updatedAt":1,"game":{"name":"EA SPORTS FC 26","shopUri":"https://ec.nintendo.com/apps/0400c460219b2000/FR?acdIndex=0","totalPlayTime":1697}}},
		{"nsaId":"b2","name":"Away","presence":{"state":"INACTIVE","game":{}}}]}}`})
	fr, err := c.Friends(context.Background(), "SESSION")
	if err != nil || len(fr) != 2 {
		t.Fatalf("friends=%v err=%v", fr, err)
	}
	if !fr[0].Playing() || fr[1].Playing() {
		t.Fatal("ONLINE with a game is playing, INACTIVE is not")
	}
	if TitleID(fr[0].Presence.Game.ShopURI) != "0400c460219b2000" {
		t.Fatalf("title id = %q", TitleID(fr[0].Presence.Game.ShopURI))
	}
}

func TestNotFoundAndRevoked(t *testing.T) {
	c := fakeSidecar(t, map[string]string{
		"/v4/FriendRequest/Received/List": `HTTP500:{"error":"invalid_grant","error_message":"session token revoked"}`,
		"/v4/Friend/List":                 `HTTP500:{"error":"unknown_error","error_message":"[znc] Rate limit exceeded.","data":{"status":9599}}`,
	})
	if _, err := c.Friends(context.Background(), "SESSION"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limit: %v", err)
	}
	if _, err := c.LookupFriendCode(context.Background(), "SESSION", "0000-0000-0001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown code: %v", err)
	}
	if _, err := c.PlayLog(context.Background(), "SESSION", "zz"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not a friend: %v", err)
	}
	if _, err := c.ReceivedRequests(context.Background(), "SESSION"); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("revoked: %v", err)
	}
}

func TestRequestsLookupPlayLog(t *testing.T) {
	c := fakeSidecar(t, map[string]string{
		"/v4/FriendRequest/Received/List": `{"status":0,"result":{"friendRequests":[{"id":"r1","sender":{"nsaId":"a1","name":"Ouranos"}}]}}`,
		"/v3/FriendRequest/Accept":        `{"status":0,"result":{}}`,
		"/v3/Friend/GetUserByFriendCode":  `{"status":0,"result":{"nsaId":"a1","name":"Ouranos"}}`,
		"/v4/User/PlayLog/Show":           `{"status":0,"result":[{"name":"Mario Kart 8 Deluxe","shopUri":"https://ec.nintendo.com/apps/0100152000022000/FR","totalPlayTime":17039,"firstPlayedAt":1493800000}]}`,
	})
	ctx := context.Background()
	reqs, err := c.ReceivedRequests(ctx, "SESSION")
	if err != nil || len(reqs) != 1 || reqs[0].Sender.NsaID != "a1" {
		t.Fatalf("reqs=%+v err=%v", reqs, err)
	}
	if err := c.Accept(ctx, "SESSION", "r1"); err != nil {
		t.Fatal(err)
	}
	u, err := c.LookupFriendCode(ctx, "SESSION", "0478-9405-4990")
	if err != nil || u.NsaID != "a1" {
		t.Fatalf("u=%+v err=%v", u, err)
	}
	log, err := c.PlayLog(ctx, "SESSION", "a1")
	if err != nil || len(log) != 1 || log[0].TotalPlayTime != 17039 || TitleID(log[0].ShopURI) != "0100152000022000" {
		t.Fatalf("log=%+v err=%v", log, err)
	}
}

func TestNormalizeFriendCode(t *testing.T) {
	for in, want := range map[string]string{
		"SW-0478-9405-4990": "0478-9405-4990", "sw-0478-9405-4990": "0478-9405-4990",
		"0478 9405 4990": "0478-9405-4990", "047894054990": "0478-9405-4990", " SW 0478-9405-4990 ": "0478-9405-4990",
	} {
		if got, err := NormalizeFriendCode(in); err != nil || got != want {
			t.Errorf("%q -> %q %v, want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "SW-0478-9405", "0478-9405-49901", "Ouranos", "SW-0478-94O5-4990"} {
		if _, err := NormalizeFriendCode(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLoginAndRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("session_token_code") != "CODE" || r.Form.Get("session_token_code_verifier") == "" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":"invalid_request"}`)
			return
		}
		fmt.Fprint(w, `{"session_token":"LONG"}`)
	}))
	defer srv.Close()
	a := NewAccounts()
	a.Base = srv.URL
	l := a.NewLogin()
	if !strings.Contains(l.URL, "session_token_code_challenge="+Challenge(l.Verifier)) {
		t.Fatal("the URL must carry the verifier's S256 challenge")
	}
	code, state, err := ParseRedirect("npf71b963c1b7b6d119://auth#session_token_code=CODE&state=" + l.State + "&session_state=x")
	if err != nil || code != "CODE" || state != l.State {
		t.Fatalf("code=%q state=%q err=%v", code, state, err)
	}
	if _, _, err := ParseRedirect("https://example.com/#session_token_code=CODE&state=s"); err == nil {
		t.Fatal("a foreign link must be refused")
	}
	tok, err := a.Exchange(context.Background(), code, l.Verifier)
	if err != nil || tok != "LONG" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
}
