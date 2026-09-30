package ws

import "testing"

func TestPlatformOf(t *testing.T) {
	for src, want := range map[string]string{"psn_api": "playstation", "xbox_api": "xbox", "client": "", "": "", "steam_api": ""} {
		if got := PlatformOf(src); got != want {
			t.Errorf("PlatformOf(%q) = %q, want %q", src, got, want)
		}
	}
}
