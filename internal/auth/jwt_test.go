package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret")

func TestAccessTokenRoundTrip(t *testing.T) {
	in := Claims{UserID: "user-1", DeviceID: "device-1", Role: "admin"}

	raw, err := NewAccessToken(testSecret, in)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	out, err := ParseAccessToken(testSecret, raw)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if out != in {
		t.Fatalf("claims mismatch: got %+v want %+v", out, in)
	}
}

func TestParseAccessTokenRejectsWrongSecret(t *testing.T) {
	raw, _ := NewAccessToken(testSecret, Claims{UserID: "u", DeviceID: "d", Role: "member"})
	if _, err := ParseAccessToken([]byte("other-secret"), raw); err == nil {
		t.Fatal("token signed with another secret was accepted")
	}
}

func TestParseAccessTokenRejectsExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		Role: "member",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "u",
			IssuedAt:  jwt.NewNumericDate(past),
			ExpiresAt: jwt.NewNumericDate(past.Add(AccessTokenTTL)),
		},
	})
	raw, _ := tok.SignedString(testSecret)
	if _, err := ParseAccessToken(testSecret, raw); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestParseAccessTokenRejectsNoneAlgorithm(t *testing.T) {
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "u",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	raw, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := ParseAccessToken(testSecret, raw); err == nil {
		t.Fatal("alg=none token was accepted")
	}
}

func TestParseAccessTokenRejectsGarbage(t *testing.T) {
	if _, err := ParseAccessToken(testSecret, "not.a.jwt"); err == nil {
		t.Fatal("garbage token was accepted")
	}
}
