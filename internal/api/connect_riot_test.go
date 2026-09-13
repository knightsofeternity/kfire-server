package api

import (
	"strings"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
)

func TestValidPlatformAcceptsOnlyKnownRegions(t *testing.T) {
	for _, p := range riot.KnownPlatforms() {
		if !validPlatform(p) {
			t.Errorf("validPlatform(%q) = false, want true", p)
		}
	}
	for _, bad := range []string{
		"", "EUW1", "euw", "zz9", "euw1; DROP TABLE users",
		"../../etc/passwd", "euw1/../na1", "euw1%2F..",
	} {
		if validPlatform(bad) {
			t.Errorf("validPlatform(%q) = true, want false", bad)
		}
	}
}

func TestSplitRiotID(t *testing.T) {
	cases := []struct {
		in        string
		name, tag string
		ok        bool
	}{
		{"Cäps#EUW", "Cäps", "EUW", true},
		{"  Cäps#EUW  ", "Cäps", "EUW", true},
		{"Name#With#Hash", "Name#With", "Hash", true},
		{"NoHash", "", "", false},
		{"#OnlyTag", "", "", false},
		{"OnlyName#", "", "", false},
		{"", "", "", false},
		{"  #  ", "", "", false},
	}
	for _, tc := range cases {
		name, tag, ok := splitRiotID(tc.in)
		if ok != tc.ok || name != tc.name || tag != tc.tag {
			t.Errorf("splitRiotID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.in, name, tag, ok, tc.name, tc.tag, tc.ok)
		}
	}
}

func TestSplitRiotIDRejectsAnOverlongInput(t *testing.T) {
	long := strings.Repeat("a", 200) + "#EUW"
	if _, _, ok := splitRiotID(long); ok {
		t.Error("an overlong Riot ID must be refused before it reaches Riot")
	}
}
