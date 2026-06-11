package battlenet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSC2Profile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/sc2/player/12345":
			w.Write([]byte(`[{"profileId":"222","regionId":2,"realmId":1,"displayName":"Toon"}]`))
		case strings.HasPrefix(r.URL.Path, "/sc2/profile/2/1/222"):
			w.Write([]byte(`{"summary":{"displayName":"Toon"},
				"career":{"primaryRace":"Zerg","current1v1LeagueName":"Diamond"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New("id", "secret")
	c.APIBase = srv.URL

	p, err := c.SC2Profile(context.Background(), "tok", "12345", "eu")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.DisplayName != "Toon" || p.Race != "Zerg" || p.League != "Diamond" {
		t.Fatalf("unexpected sc2 profile: %+v", p)
	}
}
