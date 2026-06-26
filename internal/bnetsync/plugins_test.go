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
	want := map[string]bool{"world-of-warcraft": true, "world-of-warcraft-classic": true}
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
