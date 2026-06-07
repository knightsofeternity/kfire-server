package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// RefreshTokenTTL is the lifetime of a refresh token. Tokens are single-use:
// every successful refresh rotates the token and resets this window.
const RefreshTokenTTL = 30 * 24 * time.Hour

// NewRefreshToken generates an opaque refresh token (256 bits of entropy)
// and its SHA-256 hash. Only the hash is ever stored server-side.
func NewRefreshToken() (token string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken returns the SHA-256 digest used as the storage key.
func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
