package riot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
		body, ok := routes[r.URL.EscapedPath()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c := New("RGAPI-test")
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

func TestAccountByRiotID(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/riot/account/v1/accounts/by-riot-id/C%C3%A4ps/EUW": `{"puuid":"P1","gameName":"Cäps","tagLine":"EUW"}`,
	})
	acc, err := c.AccountByRiotID(context.Background(), "Cäps", "EUW")
	if err != nil {
		t.Fatalf("AccountByRiotID: %v", err)
	}
	if acc.PUUID != "P1" {
		t.Errorf("PUUID = %q, want P1", acc.PUUID)
	}
}

func TestAccountByRiotIDUnknownIsNotFound(t *testing.T) {
	c := fakeRiot(t, map[string]string{})
	_, err := c.AccountByRiotID(context.Background(), "Nobody", "XXXX")
	if !NotFound(err) {
		t.Errorf("an unknown Riot ID must surface as NotFound, got %v", err)
	}
}

func TestAccountByRiotIDEscapesBothSegments(t *testing.T) {
	// A slash in either half must not escape the path and hit another route.
	c := fakeRiot(t, map[string]string{})
	_, err := c.AccountByRiotID(context.Background(), "a/b", "c/d")
	if err == nil {
		t.Fatal("want an error, the fake serves no such route")
	}
	var e *APIError
	if !asAPIError(err, &e) {
		t.Fatalf("want an APIError, got %T", err)
	}
	if strings.Contains(e.Path, "a/b") || strings.Contains(e.Path, "c/d") {
		t.Errorf("path %q kept a raw slash; both segments must be escaped", e.Path)
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
