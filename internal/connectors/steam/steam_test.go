package steam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthURL(t *testing.T) {
	c := New("key")
	got := c.AuthURL("https://kfire.example.org/api/v1/connect/steam/callback?state=abc",
		"https://kfire.example.org")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("AuthURL not a URL: %v", err)
	}
	q := u.Query()
	if q.Get("openid.mode") != "checkid_setup" {
		t.Errorf("mode = %q", q.Get("openid.mode"))
	}
	if q.Get("openid.identity") != identifierSelect {
		t.Errorf("identity = %q", q.Get("openid.identity"))
	}
	if !strings.Contains(q.Get("openid.return_to"), "state=abc") {
		t.Errorf("return_to lost the state: %q", q.Get("openid.return_to"))
	}
}

func validCallbackParams() url.Values {
	return url.Values{
		"openid.ns":         {openidNS},
		"openid.mode":       {"id_res"},
		"openid.claimed_id": {"https://steamcommunity.com/openid/id/76561197960287930"},
		"openid.identity":   {"https://steamcommunity.com/openid/id/76561197960287930"},
		"openid.sig":        {"somesignature"},
		"state":             {"should-not-be-echoed"},
	}
}

func TestVerifyCallback_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("openid.mode") != "check_authentication" {
			t.Errorf("mode not switched to check_authentication: %q", r.FormValue("openid.mode"))
		}
		if r.Form.Has("state") {
			t.Error("non-openid param 'state' should not be echoed to Steam")
		}
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
	}))
	defer srv.Close()

	c := New("key")
	c.LoginBase = srv.URL

	steamID, err := c.VerifyCallback(context.Background(), validCallbackParams())
	if err != nil {
		t.Fatalf("VerifyCallback: %v", err)
	}
	if steamID != "76561197960287930" {
		t.Errorf("steamID = %q", steamID)
	}
}

func TestVerifyCallback_Forged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:false\n"))
	}))
	defer srv.Close()

	c := New("key")
	c.LoginBase = srv.URL
	if _, err := c.VerifyCallback(context.Background(), validCallbackParams()); err == nil {
		t.Fatal("forged assertion accepted")
	}
}

func TestVerifyCallback_BadClaimedID(t *testing.T) {
	c := New("key")
	c.LoginBase = "http://unused"
	params := validCallbackParams()
	params.Set("openid.claimed_id", "https://evil.example.com/openid/id/76561197960287930")
	if _, err := c.VerifyCallback(context.Background(), params); err == nil {
		t.Fatal("claimed_id from a non-Steam host accepted")
	}
}

func TestResolvePlayer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("steamids") != "76561197960287930" {
			t.Errorf("steamids = %q", r.URL.Query().Get("steamids"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"response":{"players":[{
			"steamid":"76561197960287930",
			"personaname":"Robin",
			"avatarfull":"https://avatars.example/robin.jpg",
			"profileurl":"https://steamcommunity.com/id/robin/"
		}]}}`))
	}))
	defer srv.Close()

	c := New("key")
	c.APIBase = srv.URL

	p, err := c.ResolvePlayer(context.Background(), "76561197960287930")
	if err != nil {
		t.Fatalf("ResolvePlayer: %v", err)
	}
	if p.PersonaName != "Robin" || p.AvatarURL == "" || p.ProfileURL == "" {
		t.Errorf("unexpected player: %+v", p)
	}
}
