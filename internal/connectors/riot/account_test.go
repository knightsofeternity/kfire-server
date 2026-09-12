package riot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeRiot serves both the account cluster and the platform hosts from one
// httptest server; the connector's host template points every host at it.
func fakeRiot(t *testing.T, routes map[string]string) *Connector {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Riot-Token"); got != "RGAPI-test" {
			t.Errorf("missing API key header, got %q", got)
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c := New("id", "secret", "RGAPI-test")
	// %s swallows the host segment; every host resolves to the fake.
	c.APIHostTmpl = srv.URL + "/%s"
	return c
}

func TestAccountByPUUID(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/riot/account/v1/accounts/by-puuid/P1": `{"puuid":"P1","gameName":"Cäps","tagLine":"EUW"}`,
	})
	acc, err := c.AccountByPUUID(context.Background(), "P1")
	if err != nil {
		t.Fatalf("AccountByPUUID: %v", err)
	}
	if acc.RiotID() != "Cäps#EUW" {
		t.Errorf("RiotID() = %q, want Cäps#EUW", acc.RiotID())
	}
}

func TestActiveRegion(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/riot/account/v1/region/by-game/lol/by-puuid/P1": `{"puuid":"P1","game":"lol","region":"euw1"}`,
	})
	got, err := c.ActiveRegion(context.Background(), "P1")
	if err != nil {
		t.Fatalf("ActiveRegion: %v", err)
	}
	if got != "euw1" {
		t.Errorf("ActiveRegion = %q, want euw1", got)
	}
}

func TestActiveRegionOn404ReturnsAnError(t *testing.T) {
	c := fakeRiot(t, map[string]string{})
	if _, err := c.ActiveRegion(context.Background(), "nobody"); err == nil {
		t.Fatal("ActiveRegion on a 404 returned no error")
	}
}

func TestActiveRegionOnEmptyRegionReturnsAnError(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/riot/account/v1/region/by-game/lol/by-puuid/P1": `{"puuid":"P1","game":"lol","region":""}`,
	})
	if _, err := c.ActiveRegion(context.Background(), "P1"); err == nil {
		t.Fatal("an empty active region must be refused, it would route every later call wrong")
	}
}

func TestNotFoundRecognisesA404AndNothingElse(t *testing.T) {
	c := fakeRiot(t, map[string]string{})
	_, err := c.ActiveRegion(context.Background(), "nobody")
	if !NotFound(err) {
		t.Errorf("NotFound(404 error) = false, want true")
	}
	if NotFound(nil) {
		t.Error("NotFound(nil) = true, want false")
	}
	if NotFound(context.Canceled) {
		t.Error("NotFound(a non-API error) = true, want false")
	}
}
