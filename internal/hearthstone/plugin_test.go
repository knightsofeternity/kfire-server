package hearthstone

import (
	"context"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/gameplugin"
)

var _ gameplugin.Plugin = (*Plugin)(nil)

func TestPluginIdentity(t *testing.T) {
	p := New(nil)
	if p.ID() != "hearthstone" {
		t.Errorf("ID() = %q, want hearthstone", p.ID())
	}
	if got := p.Slugs(); len(got) != 1 || got[0] != "hearthstone" {
		t.Errorf("Slugs() = %v", got)
	}
}

func TestPluginIsAlwaysAvailable(t *testing.T) {
	// Hearthstone has no connector and no API key: the data comes from the
	// desktop client. Availability therefore never depends on configuration.
	if !New(nil).Available() {
		t.Error("Available() must be true; there is no credential to check")
	}
}

func TestRefreshDoesNothing(t *testing.T) {
	// There is nothing to crawl. A nil store proves it: if Refresh touched the
	// database this would panic.
	New(nil).Refresh(context.Background(), "some-user", "hearthstone")
}
