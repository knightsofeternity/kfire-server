package rocketleague

import (
	"context"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/gameplugin"
)

var _ gameplugin.Plugin = (*Plugin)(nil)

func TestPluginIdentity(t *testing.T) {
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

// Rocket League has no credential layer: the data comes from the member's
// own machine. The plugin is therefore always available, and the registry's
// admin switch remains the only way to turn it off.
func TestPluginHasNoConnector(t *testing.T) {
	p := New(nil)
	if p.Connector() != "" {
		t.Errorf("Connector() = %q, want \"\"", p.Connector())
	}
	if !p.Available() {
		t.Errorf("Available() = false, want true")
	}
}

// Refresh must do nothing, and above all must not dereference the store: it
// is called by the profile prefetch for every active plugin.
func TestRefreshIsInert(t *testing.T) {
	New(nil).Refresh(context.Background(), "u1", "rocket-league")
}
