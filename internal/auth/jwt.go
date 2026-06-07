package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AccessTokenTTL is the lifetime of an access token (spec: 15 minutes).
const AccessTokenTTL = 15 * time.Minute

const issuer = "kfire"

// ErrInvalidToken is returned for any unparseable, tampered or expired token.
var ErrInvalidToken = errors.New("invalid or expired access token")

// Claims is the authenticated identity carried by an access token.
type Claims struct {
	UserID   string
	DeviceID string
	Role     string
}

type jwtClaims struct {
	DeviceID string `json:"did"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// NewAccessToken signs a 15-minute HS256 access token.
func NewAccessToken(secret []byte, c Claims) (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		DeviceID: c.DeviceID,
		Role:     c.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   c.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	})
	return tok.SignedString(secret)
}

// ParseAccessToken verifies signature, issuer and expiry, and returns the claims.
func ParseAccessToken(secret []byte, raw string) (Claims, error) {
	var claims jwtClaims
	tok, err := jwt.ParseWithClaims(raw, &claims,
		func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return secret, nil
		},
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !tok.Valid {
		return Claims{}, ErrInvalidToken
	}
	return Claims{UserID: claims.Subject, DeviceID: claims.DeviceID, Role: claims.Role}, nil
}
