package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/auth"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// passwordResetTTL bounds how long an admin-generated reset link stays valid.
const passwordResetTTL = time.Hour

func randResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashResetToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// POST /api/v1/admin/members/:id/reset  (admin)
//
// Generates a single-use password reset link for a member. No email is sent:
// the admin shares the returned URL with the member directly.
func (h *handlers) adminResetPassword(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, err := h.store.GetUserByID(c.Context(), id); errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "user not found")
	} else if err != nil {
		return err
	}

	token, err := randResetToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(passwordResetTTL)
	if err := h.store.CreatePasswordReset(c.Context(), id, hashResetToken(token), expiresAt); err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"reset_url":  h.cfg.PublicURL + "/reset/" + token,
		"expires_at": expiresAt.UTC(),
	})
}

// GET /api/v1/auth/reset/:token  (public)
//
// Confirms a reset link is valid and whose account it is, so the page can show
// the username before the member sets a new password.
func (h *handlers) peekReset(c *fiber.Ctx) error {
	_, username, err := h.store.PeekPasswordReset(c.Context(), hashResetToken(c.Params("token")))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "invalid_token", "this reset link is invalid or expired")
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"username": username})
}

// POST /api/v1/auth/reset/:token  (public)
//
// Sets a new password from a valid reset link, then revokes the member's
// existing sessions.
func (h *handlers) doReset(c *fiber.Ctx) error {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}

	hash := hashResetToken(c.Params("token"))
	userID, username, err := h.store.PeekPasswordReset(c.Context(), hash)
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "invalid_token", "this reset link is invalid or expired")
	}
	if err != nil {
		return err
	}

	if len(req.Password) < 12 || len(req.Password) > 128 {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"password must be 12-128 characters")
	}
	if weakPassword(req.Password, username, "") {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "weak_password",
			"password is too common or based on your name - choose a stronger one")
	}

	pwHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	// Consume the token before writing, so a double-submit can't reuse it.
	if _, err := h.store.ConsumePasswordReset(c.Context(), hash); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errorJSON(c, fiber.StatusNotFound, "invalid_token", "this reset link is invalid or expired")
		}
		return err
	}
	if err := h.store.SetUserPassword(c.Context(), userID, pwHash); err != nil {
		return err
	}
	_ = h.store.DeleteUserRefreshTokens(c.Context(), userID)
	return c.SendStatus(fiber.StatusNoContent)
}
