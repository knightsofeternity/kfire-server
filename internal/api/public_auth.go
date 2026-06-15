package api

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/knightsofeternity/kfire-server/internal/apikey"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// localsAPIKeyID is the Fiber locals slot holding the authenticated key id.
const localsAPIKeyID = "api_key_id"

// parseBearer extracts the token from an "Authorization: Bearer <token>" header.
// The scheme is case-insensitive; surrounding whitespace is trimmed.
func parseBearer(header string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	tok := strings.TrimSpace(parts[1])
	if tok == "" {
		return "", false
	}
	return tok, true
}

// requireAPIKey authenticates a public request by its API key. On success it
// stores the key id in locals and refreshes last_used_at (throttled, detached).
func (h *handlers) requireAPIKey(c *fiber.Ctx) error {
	tok, ok := parseBearer(c.Get(fiber.HeaderAuthorization))
	if !ok {
		return errorJSON(c, fiber.StatusUnauthorized, "invalid_api_key", "missing or malformed API key")
	}
	key, err := h.store.LookupAPIKey(c.Context(), apikey.Hash(tok))
	if err == store.ErrNotFound {
		return errorJSON(c, fiber.StatusUnauthorized, "invalid_api_key", "unknown or revoked API key")
	}
	if err != nil {
		return err
	}
	c.Locals(localsAPIKeyID, key.ID)
	// Refresh last_used_at without blocking the response. The id is a value copy
	// (not backed by Fiber's request buffer), so it's safe in a detached goroutine.
	id := key.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.store.TouchAPIKeyLastUsed(ctx, id)
	}()
	return c.Next()
}

// apiKeyRateLimiter limits public requests per API key (not per IP, since one
// consumer behind one IP is normal). Mount AFTER requireAPIKey so locals are set.
func apiKeyRateLimiter(max int) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			if id, ok := c.Locals(localsAPIKeyID).(string); ok && id != "" {
				return id
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderRetryAfter, "60")
			return errorJSON(c, fiber.StatusTooManyRequests, "rate_limited", "too many requests")
		},
	})
}
