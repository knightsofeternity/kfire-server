package xbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeXBL(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Authorization"); got != "usertoken" {
			t.Errorf("X-Authorization = %q, want member token %q on %s", got, "usertoken", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/account":
			w.Write([]byte(`{"profileUsers":[{"id":"2533274","settings":[{"id":"Gamertag","value":"Major Nelson"},{"id":"Gamerscore","value":"123456"}]}]}`))
		case "/api/v2/2533274/presence":
			w.Write([]byte(`[{"state":"Online","devices":[{"type":"XboxSeriesX","titles":[{"id":"1777","name":"Halo Infinite","state":"Active"}]}]}]`))
		case "/api/v2/000/presence":
			w.Write([]byte(`[{"state":"Offline","devices":[]}]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path); w.WriteHeader(404)
		}
	}))
}

func TestAccount(t *testing.T) {
	srv := fakeXBL(t); defer srv.Close()
	c := New("appkey"); c.APIBase = srv.URL
	a, err := c.Account(context.Background(), "usertoken")
	if err != nil { t.Fatal(err) }
	if a.XUID != "2533274" || a.Gamertag != "Major Nelson" || a.Gamerscore != 123456 {
		t.Fatalf("unexpected account: %+v", a)
	}
}

func TestPresencePlaying(t *testing.T) {
	srv := fakeXBL(t); defer srv.Close()
	c := New("appkey"); c.APIBase = srv.URL
	p, err := c.Presence(context.Background(), "usertoken", "2533274")
	if err != nil { t.Fatal(err) }
	if !p.Playing || p.TitleID != "1777" || p.TitleName != "Halo Infinite" {
		t.Fatalf("unexpected presence: %+v", p)
	}
}

func TestPresenceNotPlaying(t *testing.T) {
	srv := fakeXBL(t); defer srv.Close()
	c := New("appkey"); c.APIBase = srv.URL
	p, err := c.Presence(context.Background(), "usertoken", "000")
	if err != nil { t.Fatal(err) }
	if p.Playing { t.Fatalf("expected not playing: %+v", p) }
}

func TestExchangeCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app/claim" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path); w.WriteHeader(404); return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]string
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["code"] != "thecode" || body["app_key"] != "pubkey" {
			t.Errorf("unexpected claim body: %s", strings.TrimSpace(string(raw)))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"app_key":"MEMBERKEY"}`))
	}))
	defer srv.Close()
	c := New("appkey"); c.ClaimBase = srv.URL; c.APIBase = srv.URL
	key, err := c.ExchangeCode(context.Background(), "thecode", "pubkey")
	if err != nil { t.Fatal(err) }
	if key != "MEMBERKEY" { t.Fatalf("ExchangeCode = %q, want MEMBERKEY", key) }
}
