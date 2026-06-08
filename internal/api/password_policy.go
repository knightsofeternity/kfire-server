package api

import "strings"

// Password policy follows current NIST guidance: favour length over forced
// composition, but reject the predictable passwords credential-stuffing tries
// first — the most common base words (with any trailing digits/symbols) and
// anything derived from the account's own identifiers.

// commonBases are the most-abused password "stems". We strip trailing digits
// and punctuation before matching, so "password", "Password1", "passw0rd!!"
// and "azertyuiop123" all collapse to a blocked base.
var commonBases = map[string]struct{}{}

func init() {
	for _, p := range []string{
		"password", "passw0rd", "p@ssw0rd", "motdepasse", "azerty", "qwerty",
		"azertyuiop", "qwertyuiop", "iloveyou", "letmein", "welcome", "changeme",
		"football", "baseball", "superman", "starwars", "whatever", "trustno",
		"admin", "administrator", "monkey", "dragon", "sunshine", "princess",
		"abcdefghijkl", "123456789012", "qwertyuiop123",
	} {
		commonBases[p] = struct{}{}
	}
}

// weakPassword reports whether a password is predictable: a common base word
// (ignoring trailing digits/symbols), derived from the username/email, or a
// single repeated character.
func weakPassword(password, username, email string) bool {
	p := strings.ToLower(password)

	if _, bad := commonBases[stripTrailing(p)]; bad {
		return true
	}
	if _, bad := commonBases[p]; bad {
		return true
	}

	// Derived from identifiers (e.g. "robin@org" → "robin12345").
	if u := strings.ToLower(username); len(u) >= 4 && strings.Contains(p, u) {
		return true
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && len(local) >= 4 && strings.Contains(p, local) {
		return true
	}

	// A single repeated rune ("aaaaaaaaaaaa").
	return isSingleRune(password)
}

// stripTrailing removes trailing digits and common punctuation so that
// "password1234!" reduces to "password".
func stripTrailing(s string) string {
	return strings.TrimRight(s, "0123456789!@#$%^&*._-")
}

func isSingleRune(s string) bool {
	if s == "" {
		return false
	}
	first := []rune(s)[0]
	for _, r := range s {
		if r != first {
			return false
		}
	}
	return true
}
