package battlenet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthURLRequestsProfileScopes(t *testing.T) {
	c := New("id", "secret")
	u := c.AuthURL("state123", "https://kfire/cb")
	for _, want := range []string{"openid", "wow.profile", "sc2.profile", "d3.profile"} {
		if !strings.Contains(u, want) {
			t.Errorf("AuthURL missing scope %q: %s", want, u)
		}
	}
}

func TestExchangeCodeCapturesScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"tok","token_type":"bearer","expires_in":86399,"scope":"openid wow.profile"}`))
	}))
	defer srv.Close()

	c := New("id", "secret")
	c.OAuthBase = srv.URL
	tok, err := c.ExchangeCode(context.Background(), "code", "https://kfire/cb")
	if err != nil {
		t.Fatal(err)
	}
	if tok.Scope != "openid wow.profile" {
		t.Errorf("scope = %q, want %q", tok.Scope, "openid wow.profile")
	}
}
