package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrevoEnvoie(t *testing.T) {
	var got map[string]any
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/smtp/email" || r.Method != http.MethodPost {
			t.Errorf("appel inattendu %s %s", r.Method, r.URL.Path)
		}
		key = r.Header.Get("api-key")
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"messageId":"<x@smtp-relay>"}`))
	}))
	defer srv.Close()

	b := NewBrevo("cle-secrete", "no-reply@guilde-ke.fr", "KFIRE")
	b.BaseURL = srv.URL
	err := b.Send(context.Background(), Message{
		To: "membre@example.test", ToName: "Djam", Subject: "Objet", Text: "texte", HTML: "<p>html</p>",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if key != "cle-secrete" {
		t.Fatalf("en-tete api-key %q", key)
	}
	sender := got["sender"].(map[string]any)
	to := got["to"].([]any)[0].(map[string]any)
	if sender["email"] != "no-reply@guilde-ke.fr" || sender["name"] != "KFIRE" ||
		to["email"] != "membre@example.test" || to["name"] != "Djam" ||
		got["subject"] != "Objet" || got["textContent"] != "texte" || got["htmlContent"] != "<p>html</p>" {
		t.Fatalf("corps inattendu : %v", got)
	}
}

func TestBrevoErreurSansFuite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":"unauthorized","message":"unrecognised IP address"}`))
	}))
	defer srv.Close()

	b := NewBrevo("cle-secrete", "no-reply@guilde-ke.fr", "KFIRE")
	b.BaseURL = srv.URL
	err := b.Send(context.Background(), Message{To: "membre@example.test", Subject: "s", Text: "t", HTML: "h"})
	if err == nil {
		t.Fatal("une reponse 401 doit etre une erreur")
	}
	msg := err.Error()
	if !strings.Contains(msg, "401") || !strings.Contains(msg, "unauthorized") {
		t.Fatalf("l erreur doit porter le code : %q", msg)
	}
	if strings.Contains(msg, "cle-secrete") || strings.Contains(msg, "membre@example.test") {
		t.Fatalf("l erreur ne doit contenir ni la cle ni l adresse : %q", msg)
	}
}
