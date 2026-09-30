package psn

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
	c := New()
	c.AuthBase, c.APIBase, c.ProfBase = srv.URL+"/oauth", srv.URL+"/api", srv.URL+"/prof"
	return c
}

func TestExchangeGetsTokens(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/authorize":
			if !strings.Contains(r.Header.Get("Cookie"), "npsso=good") {
				t.Errorf("cookie = %q", r.Header.Get("Cookie"))
			}
			w.Header().Set("Location", "com.scee.psxandroid.scecompcall://redirect?code=abc")
			w.WriteHeader(http.StatusFound)
		case "/oauth/token":
			r.ParseForm()
			if r.Form.Get("code") != "abc" {
				t.Errorf("code = %q", r.Form.Get("code"))
			}
			fmt.Fprint(w, `{"access_token":"A","expires_in":3599,"refresh_token":"R","refresh_token_expires_in":863999}`)
		}
	})
	tok, err := c.Exchange(context.Background(), "good")
	if err != nil || tok.Access != "A" || tok.Refresh != "R" {
		t.Fatalf("tok=%+v err=%v", tok, err)
	}
}

func TestExchangeRevokedNPSSO(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://my.account.sony.com/sonyacct/signin/?error=login_required&error_code=4165")
		w.WriteHeader(http.StatusFound)
	})
	if _, err := c.Exchange(context.Background(), "dead"); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("err = %v, want ErrLoginRequired", err)
	}
}

func TestLookupAndForbidden(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/prof/Balder_ke/"):
			fmt.Fprint(w, `{"profile":{"accountId":"111","onlineId":"Balder_ke"}}`)
		case strings.HasPrefix(r.URL.Path, "/prof/"):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	})
	p, err := c.Lookup(context.Background(), "T", "Balder_ke")
	if err != nil || p.AccountID != "111" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if _, err := c.Lookup(context.Background(), "T", "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := c.PlayedGames(context.Background(), "T", "111"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("not a friend: %v", err)
	}
}

func TestPlayedGamesPaginates(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "0" {
			var b strings.Builder
			b.WriteString(`{"totalItemCount":201,"titles":[`)
			for i := 0; i < 200; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `{"titleId":"T%d","name":"G%d","playDuration":"PT1H","concept":{"id":%d,"titleIds":["T%d"]}}`, i, i, i, i)
			}
			b.WriteString(`]}`)
			fmt.Fprint(w, b.String())
			return
		}
		fmt.Fprint(w, `{"totalItemCount":201,"titles":[{"titleId":"PPSA08595_00","name":"Diablo® IV","playDuration":"PT626H29M12S","concept":{"id":10002694,"titleIds":["PPSA08595_00","CUSA35050_00"]}}]}`)
	})
	games, err := c.PlayedGames(context.Background(), "T", "222")
	if err != nil || len(games) != 201 {
		t.Fatalf("len=%d err=%v", len(games), err)
	}
	d := games[200]
	if d.Seconds != 626*3600+29*60+12 || d.ConceptID != "10002694" || len(d.ConceptTitle) != 2 {
		t.Fatalf("diablo = %+v", d)
	}
}

func TestPresencesAndFriends(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/basicPresences"):
			if r.URL.Query().Get("accountIds") != "1,2" {
				t.Errorf("ids = %q", r.URL.Query().Get("accountIds"))
			}
			fmt.Fprint(w, `{"basicPresences":[
				{"accountId":"1","primaryPlatformInfo":{"onlineStatus":"online"},"gameTitleInfoList":[{"npTitleId":"PPSA08595_00","titleName":"Diablo IV"}]},
				{"accountId":"2","primaryPlatformInfo":{"onlineStatus":"offline"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/receivedRequests"):
			fmt.Fprint(w, `{"receivedRequests":[{"accountId":"9"},{"accountId":"1"}]}`)
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	ps, err := c.Presences(context.Background(), "T", []string{"1", "2"})
	if err != nil || len(ps) != 2 || ps[0].TitleID != "PPSA08595_00" || !ps[0].Online || ps[1].Online {
		t.Fatalf("ps=%+v err=%v", ps, err)
	}
	reqs, err := c.ReceivedRequests(context.Background(), "T")
	if err != nil || len(reqs) != 2 {
		t.Fatalf("reqs=%v err=%v", reqs, err)
	}
	if err := c.AcceptFriend(context.Background(), "T", "1"); err != nil {
		t.Fatal(err)
	}
}

func TestEarnedTrophiesJoinsNames(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/users/") {
			fmt.Fprint(w, `{"trophies":[{"trophyId":0,"earned":false},{"trophyId":4,"earned":true,"earnedDateTime":"2026-08-20T19:45:39Z"}]}`)
			return
		}
		fmt.Fprint(w, `{"trophies":[{"trophyId":0,"trophyName":"Victorieux"},{"trophyId":4,"trophyName":"Blindé","trophyIconUrl":"https://x/4.png","trophyType":"silver"}]}`)
	})
	got, err := c.EarnedTrophies(context.Background(), "T", "1", TrophyTitle{NpCommunicationID: "NPWR10017_00", Service: "trophy"})
	if err != nil || len(got) != 1 || got[0].Name != "Blindé" || got[0].Type != "silver" || got[0].EarnedAt.IsZero() {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

// An empty date from Sony must not cost the whole page.
func TestEmptyDatesDoNotFailADecode(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"totalItemCount":1,"titles":[{"titleId":"CUSA1","name":"Old","playDuration":"PT2H","lastPlayedDateTime":"","concept":{"id":"42"}}]}`)
	})
	games, err := c.PlayedGames(context.Background(), "T", "1")
	if err != nil || len(games) != 1 || games[0].Seconds != 7200 || games[0].ConceptID != "42" {
		t.Fatalf("games=%+v err=%v", games, err)
	}
}
