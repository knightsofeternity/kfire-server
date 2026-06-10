package api

import "testing"

func TestSessionVisibilityFor(t *testing.T) {
	const viewer, target = "u-viewer", "u-target"

	// Self always sees everything.
	if v := sessionVisibilityFor(target, "member", target, false, false); v.HideAll || v.HideOpen {
		t.Errorf("self: want all visible, got %+v", v)
	}
	// Admin always sees everything.
	if v := sessionVisibilityFor(viewer, "admin", target, false, false); v.HideAll || v.HideOpen {
		t.Errorf("admin: want all visible, got %+v", v)
	}
	// Other member, both toggles on: nothing hidden.
	if v := sessionVisibilityFor(viewer, "member", target, true, true); v.HideAll || v.HideOpen {
		t.Errorf("visible target: got %+v", v)
	}
	// Other member, sessions hidden: HideAll.
	if v := sessionVisibilityFor(viewer, "member", target, true, false); !v.HideAll {
		t.Errorf("sessions hidden: want HideAll, got %+v", v)
	}
	// Other member, activity hidden only: HideOpen, not HideAll.
	if v := sessionVisibilityFor(viewer, "member", target, false, true); v.HideAll || !v.HideOpen {
		t.Errorf("activity hidden: want HideOpen only, got %+v", v)
	}
}
