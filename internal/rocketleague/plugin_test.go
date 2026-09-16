package rocketleague

import (
	"context"
	"testing"
)

func TestPluginIdentite(t *testing.T) {
	p := New(nil)
	if p.ID() != "rocket-league" {
		t.Errorf("ID() = %q, want \"rocket-league\"", p.ID())
	}
	if p.Name() != "Rocket League" {
		t.Errorf("Name() = %q, want \"Rocket League\"", p.Name())
	}
	if got := p.Slugs(); len(got) != 1 || got[0] != "rocket-league" {
		t.Errorf("Slugs() = %v, want [rocket-league]", got)
	}
}

// Rocket League n'a aucune couche de credentials : la donnée vient de la machine
// du membre. Le plugin est donc toujours disponible, et c'est l'interrupteur
// admin du registre qui reste le seul moyen de l'éteindre.
func TestPluginSansConnecteur(t *testing.T) {
	p := New(nil)
	if p.Connector() != "" {
		t.Errorf("Connector() = %q, want \"\"", p.Connector())
	}
	if !p.Available() {
		t.Errorf("Available() = false, want true")
	}
}

// Refresh ne doit rien faire, et surtout ne pas déréférencer le store : il est
// appelé par le prefetch du profil pour tout plugin actif.
func TestRefreshInerte(t *testing.T) {
	New(nil).Refresh(context.Background(), "u1", "rocket-league")
}
