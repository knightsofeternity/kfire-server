package api

import "testing"

func TestParseBearer(t *testing.T) {
	cases := []struct {
		header string
		want   string
		ok     bool
	}{
		{"Bearer kfire_abc", "kfire_abc", true},
		{"bearer kfire_abc", "kfire_abc", true}, // scheme is case-insensitive
		{"Bearer  kfire_abc ", "kfire_abc", true}, // trims surrounding spaces
		{"kfire_abc", "", false},                  // missing scheme
		{"Bearer ", "", false},                    // empty token
		{"", "", false},                           // no header
		{"Basic xyz", "", false},                  // wrong scheme
	}
	for _, tc := range cases {
		got, ok := parseBearer(tc.header)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseBearer(%q) = (%q,%v), want (%q,%v)", tc.header, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPublicViewerIsNonPrivileged(t *testing.T) {
	// A member who hid both toggles must be fully hidden from the public viewer.
	v := sessionVisibilityFor("", publicViewerRole, "target", false, false)
	if !v.HideAll || !v.HideOpen {
		t.Errorf("public viewer must not see hidden member: got %+v", v)
	}
	// A member who shares both toggles is fully visible.
	v = sessionVisibilityFor("", publicViewerRole, "target", true, true)
	if v.HideAll || v.HideOpen {
		t.Errorf("public viewer should see opted-in member: got %+v", v)
	}
}
