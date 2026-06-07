package api

import (
	"github.com/gofiber/fiber/v2"
)

// GET /api/v1/users/me  (authenticated)
func (h *handlers) me(c *fiber.Ctx) error {
	u, err := h.store.GetUserByID(c.Context(), mustClaims(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(userJSON(u, true))
}
