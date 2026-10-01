package bnetsync

import (
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/battlenet"
	"github.com/knightsofeternity/kfire-server/internal/gameplugin"
)

// Compile-time proof the plugins satisfy the interface.
var (
	_ gameplugin.Plugin = (*WowPlugin)(nil)
	_ gameplugin.Plugin = (*BnetProfilePlugin)(nil)
)

func TestWowPluginMetadata(t *testing.T) {
	conn := battlenet.New("", "") // disabled (no creds)
	p := NewWowPlugin(nil, nil, conn)
	if p.ID() != "wow" || p.Name() != "World of Warcraft" || p.Connector() != "battlenet" {
		t.Fatalf("metadata wrong: %s %s %s", p.ID(), p.Name(), p.Connector())
	}
	if p.Available() {
		t.Fatalf("wow should be unavailable with empty bnet creds")
	}
	want := map[string]bool{"world-of-warcraft": true, "world-of-warcraft-classic": true, "world-of-warcraft-forever": true, "wow-ascension": true}
	for _, s := range p.Slugs() {
		if !want[s] {
			t.Fatalf("unexpected slug %q", s)
		}
		delete(want, s)
	}
	if len(want) != 0 {
		t.Fatalf("missing slugs: %v", want)
	}
}

func TestWowPluginAvailableWithCreds(t *testing.T) {
	conn := battlenet.New("id", "secret")
	p := NewWowPlugin(nil, nil, conn)
	if !p.Available() {
		t.Fatalf("wow should be available when bnet creds are set")
	}
}

func TestBnetProfilePluginMetadata(t *testing.T) {
	conn := battlenet.New("id", "secret")
	d3 := NewBnetProfilePlugin(nil, nil, conn, "d3", "Diablo III", "diablo-iii")
	if d3.ID() != "d3" || d3.Name() != "Diablo III" || d3.Connector() != "battlenet" {
		t.Fatalf("d3 metadata wrong")
	}
	if len(d3.Slugs()) != 1 || d3.Slugs()[0] != "diablo-iii" {
		t.Fatalf("d3 slugs wrong: %v", d3.Slugs())
	}
	if !d3.Available() {
		t.Fatalf("d3 should be available with creds")
	}
}

// A Battle.net realm and the addon's realm must meet: slug, display name and
// the addon's name differ in case, quotes and spaces.
func TestRealmKeyMeetsAcrossSources(t *testing.T) {
	addon := realmKey("Cho'gall", "Ouranos")
	for _, bnet := range [][2]string{{"chogall", "Ouranos"}, {"Cho'gall", "ouranos"}, {"Cho’gall", "OURANOS"}} {
		if got := realmKey(bnet[0], bnet[1]); got != addon {
			t.Errorf("realmKey(%q, %q) = %q, want %q", bnet[0], bnet[1], got, addon)
		}
	}
	if realmKey("Confrérie du Thorium", "X") != realmKey("ConfrérieduThorium", "x") {
		t.Error("spaces must not matter")
	}
	if wowClassName("DEATHKNIGHT") != "Death Knight" || wowClassName("PRIEST") != "Priest" {
		t.Error("class tokens must read like Battle.net class names")
	}
	if wowClassName("SONOFARUGAL") != "Son of Arugal" || wowClassName("CHRONOMANCER") != "Chronomancer" {
		t.Error("Ascension's own classes must read as names")
	}
	if wowClassName("LIGHTBRINGER") != "Lightbringer" || wowClassName("") != "" {
		t.Error("an unknown class must read as a word, not a token")
	}
}
