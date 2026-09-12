package api

import (
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
