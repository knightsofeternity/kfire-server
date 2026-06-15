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
