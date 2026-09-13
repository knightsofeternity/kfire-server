package bnetsync

import "strings"

// The game version a character belongs to. Stored on each character row,
// because the catalog slug "world-of-warcraft-classic" covers several
// incompatible versions at once and their numbers cannot be compared.
const (
	VersionRetail             = "retail"
	VersionClassicEra         = "classic-era"
	VersionClassicProgression = "classic-progression"
)

// versionFromNamespace maps a Blizzard profile namespace to a version.
//
// Order matters: "profile-classic1x-eu" also starts with "profile-classic", so
// the Era case must be tested before the progression case or Era realms would
// be filed as progression.
//
// An unrecognised namespace yields an empty string rather than a guess. The
// column is nullable and the interface files empty versions under a neutral
// group, which is honest; inventing a version would not be.
func versionFromNamespace(ns string) string {
	switch {
	case strings.HasPrefix(ns, "profile-classic1x-"):
		return VersionClassicEra
	case strings.HasPrefix(ns, "profile-classic-"):
		return VersionClassicProgression
	case strings.HasPrefix(ns, "profile-"):
		return VersionRetail
	default:
		return ""
	}
}
