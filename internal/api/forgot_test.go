package api

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/mail"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

type fakeForgotStore struct {
	users  map[string]store.User
	resets []string
}

func (f *fakeForgotStore) GetUserByLogin(_ context.Context, login string) (store.User, error) {
	for _, u := range f.users {
		if u.Username == login || u.Email == login {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeForgotStore) CreatePasswordReset(_ context.Context, userID string, _ []byte, _ time.Time) error {
	f.resets = append(f.resets, userID)
	return nil
}

type fakeSender struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (f *fakeSender) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

func newForgotFixture() (*forgotService, *fakeForgotStore, *fakeSender, *time.Time) {
	banned := time.Now()
	st := &fakeForgotStore{users: map[string]store.User{
		"u1": {ID: "u1", Username: "Djam", Email: "djam@example.test"},
		"u2": {ID: "u2", Username: "Banni", Email: "banni@example.test", BannedAt: &banned},
		"u3": {ID: "u3", Username: "SansMail"},
	}}
	sender := &fakeSender{}
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	svc := newForgotService(st, sender, "https://kfire.example.test", "Knights of Eternity")
	svc.now = func() time.Time { return now }
	return svc, st, sender, &now
}

func TestForgotEnvoieAuMembre(t *testing.T) {
	svc, st, sender, _ := newForgotFixture()
	if got := svc.handle(context.Background(), "djam@example.test", "fr"); got != "sent" {
		t.Fatalf("issue %q, attendu sent", got)
	}
	if len(st.resets) != 1 || st.resets[0] != "u1" {
		t.Fatalf("jeton non cree : %v", st.resets)
	}
	m := sender.sent[0]
	if m.To != "djam@example.test" || m.ToName != "Djam" {
		t.Fatalf("destinataire inattendu : %+v", m)
	}
	if !strings.Contains(m.Subject, "Réinitialisation") || !strings.Contains(m.Text, "https://kfire.example.test/reset/") {
		t.Fatalf("e-mail inattendu : %+v", m)
	}
}

func TestForgotParPseudoEtEnAnglais(t *testing.T) {
	svc, _, sender, _ := newForgotFixture()
	if got := svc.handle(context.Background(), "Djam", "de"); got != "sent" {
		t.Fatalf("issue %q", got)
	}
	if !strings.HasPrefix(sender.sent[0].Subject, "Reset your") {
		t.Fatalf("une langue inconnue vaut l anglais : %q", sender.sent[0].Subject)
	}
}

func TestForgotNenvoieRien(t *testing.T) {
	for login, want := range map[string]string{
		"inconnu":            "no_account",
		"banni@example.test": "banned",
		"SansMail":           "no_email",
	} {
		svc, st, sender, _ := newForgotFixture()
		if got := svc.handle(context.Background(), login, "fr"); got != want {
			t.Errorf("%s : issue %q, attendu %q", login, got, want)
		}
		if len(st.resets) != 0 || len(sender.sent) != 0 {
			t.Errorf("%s : rien ne doit etre cree ni envoye", login)
		}
	}
}

func TestForgotLimiteParCompte(t *testing.T) {
	svc, _, sender, now := newForgotFixture()
	svc.handle(context.Background(), "Djam", "fr")
	*now = now.Add(14 * time.Minute)
	if got := svc.handle(context.Background(), "djam@example.test", "fr"); got != "throttled" {
		t.Fatalf("issue %q a 14 min, attendu throttled", got)
	}
	*now = now.Add(2 * time.Minute)
	if got := svc.handle(context.Background(), "Djam", "fr"); got != "sent" {
		t.Fatalf("issue %q a 16 min, attendu sent", got)
	}
	if len(sender.sent) != 2 {
		t.Fatalf("%d envois, attendu 2", len(sender.sent))
	}
}

func TestGabaritEchappeLePseudo(t *testing.T) {
	m := resetEmail("fr", "KE", `<b>x</b>`, "https://k/reset/t")
	if strings.Contains(m.HTML, "<b>x</b>") || !strings.Contains(m.HTML, "&lt;b&gt;") {
		t.Fatalf("pseudo non echappe : %s", m.HTML)
	}
	if !strings.Contains(m.Text, "<b>x</b>") {
		t.Fatal("la version texte garde le pseudo tel quel")
	}
}

func TestForgotHandlerReponseIdentique(t *testing.T) {
	svc, _, _, _ := newForgotFixture()
	h := &handlers{forgot: svc}
	app := fiber.New()
	app.Post("/forgot", h.forgotPassword)

	var bodies []string
	for _, login := range []string{"Djam", "inconnu"} {
		req := httptest.NewRequest("POST", "/forgot", strings.NewReader(`{"login":"`+login+`","lang":"fr"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 202 {
			t.Fatalf("%s : statut %d", login, res.StatusCode)
		}
		bodies = append(bodies, string(b))
	}
	if bodies[0] != bodies[1] {
		t.Fatalf("reponses differentes : %q / %q", bodies[0], bodies[1])
	}
}

func TestForgotHandlerEteint(t *testing.T) {
	h := &handlers{}
	app := fiber.New()
	app.Post("/forgot", h.forgotPassword)
	req := httptest.NewRequest("POST", "/forgot", strings.NewReader(`{"login":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	res, _ := app.Test(req)
	if res.StatusCode != 404 {
		t.Fatalf("statut %d, attendu 404", res.StatusCode)
	}
}

func TestForgotHandlerLoginVide(t *testing.T) {
	svc, _, _, _ := newForgotFixture()
	h := &handlers{forgot: svc}
	app := fiber.New()
	app.Post("/forgot", h.forgotPassword)
	req := httptest.NewRequest("POST", "/forgot", strings.NewReader(`{"login":"  "}`))
	req.Header.Set("Content-Type", "application/json")
	res, _ := app.Test(req)
	if res.StatusCode != 422 {
		t.Fatalf("statut %d, attendu 422", res.StatusCode)
	}
}
