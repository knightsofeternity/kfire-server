package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/gameplugin"
)

// GET /api/v1/admin/plugins  (admin) — every registered game plugin with its
// availability and enabled flag.
func (h *handlers) listPlugins(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"plugins": h.plugins.List()})
}

// PATCH /api/v1/admin/plugins/:id  (admin) — toggle a plugin on/off.
func (h *handlers) setPlugin(c *fiber.Ctx) error {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}
	err := h.plugins.SetEnabled(c.Context(), c.Params("id"), req.Enabled)
	if errors.Is(err, gameplugin.ErrUnknownPlugin) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "plugin not found")
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"enabled": req.Enabled})
}
