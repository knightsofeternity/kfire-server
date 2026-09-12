package riot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthURLCarriesTheStateAndScope(t *testing.T) {
	c := New("853768", "shh", "RGAPI-x")
	got := c.AuthURL("st4te", "https://kfire.example/cb")
	for _, want := range []string{
		"response_type=code", "client_id=853768", "state=st4te",
		"scope=openid", "redirect_uri=https%3A%2F%2Fkfire.example%2Fcb",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AuthURL() = %q, missing %q", got, want)
		}
	}
}

func TestExchangeCodeAndUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "853768" || pass != "shh" {
				t.Errorf("token call missing or wrong basic auth: %q %q %v", user, pass, ok)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "c0de" {
				t.Errorf("unexpected form: %v", r.Form)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":600}`))
		case "/userinfo":
			if got := r.Header.Get("Authorization"); got != "Bearer at" {
				t.Errorf("userinfo auth = %q, want Bearer at", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sub":"PUUID-123"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New("853768", "shh", "RGAPI-x")
	c.AuthBase = srv.URL

	tok, err := c.ExchangeCode(context.Background(), "c0de", "https://kfire.example/cb")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok != "at" {
		t.Fatalf("ExchangeCode = %q, want at", tok)
	}
	puuid, err := c.UserPUUID(context.Background(), tok)
	if err != nil {
		t.Fatalf("UserPUUID: %v", err)
	}
	if puuid != "PUUID-123" {
		t.Fatalf("UserPUUID = %q, want PUUID-123", puuid)
	}
}

func TestExchangeCodeSurfacesAnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	c := New("id", "secret", "key")
	c.AuthBase = srv.URL
	if _, err := c.ExchangeCode(context.Background(), "bad", "https://x/cb"); err == nil {
		t.Fatal("ExchangeCode on a 400 returned no error")
	}
}
