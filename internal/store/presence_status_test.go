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

func TestApplyPresenceOverride(t *testing.T) {
	cases := []struct {
		chosen, computed, want string
	}{
		{"invisible", "in_game", "offline"},
		{"invisible", "online", "offline"},
		{"offline", "in_game", "offline"},
		{"offline", "online", "offline"},
		{"online", "online", "online"},
		{"online", "in_game", "in_game"},
		{"online", "offline", "offline"},
		// Unknown/empty chosen status passes the computed value through unchanged.
		{"", "online", "online"},
	}
	for _, tc := range cases {
		if got := ApplyPresenceOverride(tc.chosen, tc.computed); got != tc.want {
			t.Errorf("ApplyPresenceOverride(%q, %q) = %q want %q",
				tc.chosen, tc.computed, got, tc.want)
		}
	}
}
