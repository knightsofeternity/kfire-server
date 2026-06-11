package battlenet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestD3Profile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/d3/profile/Player-1234/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"battleTag":"Player#1234","paragonLevel":1842,
			"heroes":[{"name":"Xul","class":"necromancer","level":70},
			          {"name":"Li","class":"wizard","level":70}]}`))
	}))
	defer srv.Close()
	c := New("id", "secret")
	c.APIBase = srv.URL

	p, err := c.D3Profile(context.Background(), "tok", "Player#1234", "eu")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Paragon != 1842 || len(p.Heroes) != 2 || p.Heroes[0].Name != "Xul" ||
		p.Heroes[0].Class != "necromancer" || p.Heroes[0].Level != 70 {
		t.Fatalf("unexpected d3 profile: %+v", p)
	}
}
