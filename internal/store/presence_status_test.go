package store

import "testing"

func TestPresenceStatus(t *testing.T) {
	if got := PresenceStatus(true, true, false); got != "in_game" {
		t.Errorf("open+visible: got %q want in_game", got)
	}
	if got := PresenceStatus(true, false, true); got != "online" {
		t.Errorf("open+hidden+ws: got %q want online", got)
	}
	if got := PresenceStatus(false, false, true); got != "online" {
		t.Errorf("ws only: got %q want online", got)
	}
	if got := PresenceStatus(false, false, false); got != "offline" {
		t.Errorf("none: got %q want offline", got)
	}
	if got := PresenceStatus(true, false, false); got != "offline" {
		t.Errorf("open+hidden+no-ws: got %q want offline", got)
	}
}
