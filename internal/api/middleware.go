package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/auth"
)

const claimsKey = "claims"

// requireAuth validates the Bearer access token and stores its claims in the
// request context.
func (h *handlers) requireAuth(c *fiber.Ctx) error {
	const prefix = "Bearer "
	header := c.Get(fiber.HeaderAuthorization)
	if !strings.HasPrefix(header, prefix) {
		return errorJSON(c, fiber.StatusUnauthorized, "unauthorized", "missing bearer token")
	}
	claims, err := auth.ParseAccessToken([]byte(h.cfg.JWTSecret), header[len(prefix):])
	if err != nil {
		return errorJSON(c, fiber.StatusUnauthorized, "unauthorized", "invalid or expired access token")
	}
	c.Locals(claimsKey, claims)
	return c.Next()
}

// mustClaims returns the claims set by requireAuth. Only call it from
// handlers mounted behind that middleware.
func mustClaims(c *fiber.Ctx) auth.Claims {
	return c.Locals(claimsKey).(auth.Claims)
}
