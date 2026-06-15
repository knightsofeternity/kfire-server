// Package apikey generates and hashes opaque API keys. Keys carry 256 bits of
// entropy, so a fast hash (SHA-256) is safe for at-rest storage and necessary
// for a per-request lookup; bcrypt is only for low-entropy passwords.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// prefixLen is how many leading characters we keep in cleartext to identify a
// key in listings (e.g. "kfire_AbCd…").
const prefixLen = 12

// Generate returns a new key: the full secret (shown to the admin once), its
// display prefix, and the SHA-256 hash to store. The full key looks like
// "kfire_" + 43 base64url chars (32 random bytes).
func Generate() (full, prefix string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", nil, err
	}
	full = "kfire_" + base64.RawURLEncoding.EncodeToString(buf)
	h := sha256.Sum256([]byte(full))
	return full, full[:prefixLen], h[:], nil
}

// Hash returns the SHA-256 of a presented key for lookup against the stored hash.
func Hash(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}
