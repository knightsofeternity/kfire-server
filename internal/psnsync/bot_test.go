package psnsync

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/psn"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// fakeSony accepts one NPSSO at a time (the value in good) and counts calls.
type fakeSony struct {
	good      atomic.Value
	exchanges atomic.Int32
	refreshes atomic.Int32
}

func newBot(t *testing.T) (*Bot, *fakeSony, *store.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL absent")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePsnBotForTests(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.DeletePsnBotForTests(ctx) })

	f := &fakeSony{}
	f.good.Store("good")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/authorize"):
			f.exchanges.Add(1)
			if r.Header.Get("Cookie") == "npsso="+f.good.Load().(string) {
				w.Header().Set("Location", "x://redirect?code=c")
			} else {
				w.Header().Set("Location", "https://sony/signin?error=login_required&error_code=4165")
			}
			w.WriteHeader(http.StatusFound)
		case strings.HasSuffix(r.URL.Path, "/token"):
			r.ParseForm()
			if r.Form.Get("grant_type") == "refresh_token" {
				f.refreshes.Add(1)
			}
			fmt.Fprint(w, `{"access_token":"A","expires_in":3599,"refresh_token":"R","refresh_token_expires_in":863999}`)
		case strings.HasSuffix(r.URL.Path, "/me/profile2"):
			fmt.Fprint(w, `{"profile":{"accountId":"42","onlineId":"kfirebot"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	conn := psn.New()
	conn.AuthBase, conn.APIBase, conn.ProfBase = srv.URL+"/oauth", srv.URL+"/api", srv.URL+"/prof"
	cipher, err := crypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return NewBot(st, conn, cipher), f, st, ctx
}

func TestBotTokenLifecycle(t *testing.T) {
	bot, f, st, ctx := newBot(t)

	if _, err := bot.Token(ctx); !errors.Is(err, ErrNoBot) {
		t.Fatalf("no NPSSO yet: %v", err)
	}
	if err := bot.SetNPSSO(ctx, "typo"); !errors.Is(err, psn.ErrLoginRequired) {
		t.Fatalf("a refused NPSSO must be refused at once: %v", err)
	}
	if bot.Configured(ctx) {
		t.Fatal("a refused NPSSO must not be stored")
	}
	if err := bot.SetNPSSO(ctx, "good"); err != nil {
		t.Fatal(err)
	}
	if s, _ := bot.Status(ctx); s.Status != "ok" || s.OnlineID == nil || *s.OnlineID != "kfirebot" {
		t.Fatalf("status %+v", s)
	}

	before := f.exchanges.Load()
	if tok, err := bot.Token(ctx); err != nil || tok != "A" || f.exchanges.Load() != before {
		t.Fatalf("a fresh access token must be reused: %q %v", tok, err)
	}

	// Access expired, refresh alive: refreshed, no NPSSO exchange.
	past, future := time.Now().Add(-time.Minute), time.Now().Add(48*time.Hour)
	sealA, _ := bot.cipher.SealString("A")
	sealR, _ := bot.cipher.SealString("R")
	st.SavePsnTokens(ctx, sealA, past, sealR, future, "", "")
	if _, err := bot.Token(ctx); err != nil || f.refreshes.Load() != 1 || f.exchanges.Load() != before {
		t.Fatalf("refresh path: err=%v refreshes=%d", err, f.refreshes.Load())
	}

	// Both expired and Sony revoked the NPSSO: marked, and never retried.
	st.SavePsnTokens(ctx, sealA, past, sealR, past, "", "")
	f.good.Store("another")
	if _, err := bot.Token(ctx); !errors.Is(err, ErrNeedsNPSSO) {
		t.Fatalf("revoked: %v", err)
	}
	calls := f.exchanges.Load()
	if _, err := bot.Token(ctx); !errors.Is(err, ErrNeedsNPSSO) || f.exchanges.Load() != calls {
		t.Fatal("a bot waiting for an admin must not keep hitting Sony")
	}
	if s, _ := bot.Status(ctx); s.Status != "needs_npsso" || s.LastError == nil {
		t.Fatalf("status %+v", s)
	}

	// The admin pastes the new one: back to work.
	if err := bot.SetNPSSO(ctx, "another"); err != nil {
		t.Fatal(err)
	}
	if _, err := bot.Token(ctx); err != nil {
		t.Fatal(err)
	}
}
