package bnetsync

import "testing"

func TestVersionFromNamespace(t *testing.T) {
	cases := map[string]string{
		"profile-eu":           VersionRetail,
		"profile-us":           VersionRetail,
		"profile-kr":           VersionRetail,
		"profile-classic1x-eu": VersionClassicEra,
		"profile-classic1x-us": VersionClassicEra,
		"profile-classic-eu":   VersionClassicProgression,
		"profile-classic-tw":   VersionClassicProgression,
	}
	for ns, want := range cases {
		if got := versionFromNamespace(ns); got != want {
			t.Errorf("versionFromNamespace(%q) = %q, want %q", ns, got, want)
		}
	}
}

func TestVersionFromNamespaceDoesNotConfuseEraWithProgression(t *testing.T) {
	// "profile-classic1x-eu" starts with "profile-classic", so a naive prefix
	// test would call Era a progression realm. This is the whole point of the
	// function.
	if got := versionFromNamespace("profile-classic1x-eu"); got == VersionClassicProgression {
		t.Error("Era was classified as progression; the specific case must be tested first")
	}
}

func TestVersionFromNamespaceUnknownIsEmpty(t *testing.T) {
	for _, ns := range []string{"", "profile", "static-eu", "dynamic-classic-eu"} {
		if got := versionFromNamespace(ns); got != "" {
			t.Errorf("versionFromNamespace(%q) = %q, want empty; an unknown "+
				"namespace must not be guessed", ns, got)
		}
	}
}
