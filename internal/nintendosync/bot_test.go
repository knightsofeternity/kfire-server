package nintendosync

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/nintendo"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func newBot(t *testing.T) (*Bot, *store.Store, context.Context) {
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
	st.DeleteNintendoBotForTests(ctx)
	t.Cleanup(func() { st.DeleteNintendoBotForTests(ctx) })

	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "na GOOD" {
			w.WriteHeader(500)
			fmt.Fprint(w, `{"error":"invalid_grant"}`)
			return
		}
		fmt.Fprint(w, `{"status":0,"result":{"nsaId":"botnsa","name":"KFIREKE","links":{"friendCode":{"id":"0478-9405-4990"}}}}`)
	}))
	t.Cleanup(sidecar.Close)
	accounts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("session_token_code") == "CODE" {
			fmt.Fprint(w, `{"session_token":"GOOD"}`)
			return
		}
		w.WriteHeader(400)
	}))
	t.Cleanup(accounts.Close)
	a := nintendo.NewAccounts()
	a.Base = accounts.URL
	cipher, _ := crypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	return NewBot(st, nintendo.New(sidecar.URL), a, cipher), st, ctx
}

func TestBotLoginLifecycle(t *testing.T) {
	bot, _, ctx := newBot(t)
	if _, err := bot.Session(ctx); !errors.Is(err, ErrNoBot) {
		t.Fatalf("no bot yet: %v", err)
	}
	link, err := bot.StartLogin(ctx)
	if err != nil || !strings.Contains(link, "session_token_code_challenge=") {
		t.Fatalf("link=%q err=%v", link, err)
	}
	if bot.Configured(ctx) {
		t.Fatal("a pending login is not a configured bot")
	}
	if err := bot.FinishLogin(ctx, "npf71b963c1b7b6d119://auth#session_token_code=CODE&state=forged"); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("a link for another login must be refused: %v", err)
	}
	state := strings.SplitN(strings.SplitN(link, "state=", 2)[1], "&", 2)[0]
	if err := bot.FinishLogin(ctx, "npf71b963c1b7b6d119://auth#session_token_code=CODE&state="+state); err != nil {
		t.Fatal(err)
	}
	s, _ := bot.Status(ctx)
	if !s.Configured || s.Status != "ok" || *s.Nickname != "KFIREKE" || *s.FriendCode != "0478-9405-4990" {
		t.Fatalf("status %+v", s)
	}
	if tok, err := bot.Session(ctx); err != nil || tok != "GOOD" {
		t.Fatalf("session %q %v", tok, err)
	}
	bot.Revoked(ctx)
	if _, err := bot.Session(ctx); !errors.Is(err, ErrNeedsLogin) {
		t.Fatalf("after revocation: %v", err)
	}
	if err := bot.SetSession(ctx, "BAD"); err == nil {
		t.Fatal("an unproven session must not be stored")
	}
	if err := bot.SetSession(ctx, "GOOD"); err != nil {
		t.Fatal(err)
	}
	if _, err := bot.Session(ctx); err != nil {
		t.Fatal("a new session brings the bot back")
	}
}
