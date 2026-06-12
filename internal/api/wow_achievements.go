package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

// GET /api/v1/users/:id/wow/:realm/:name/achievements  (authenticated)
// A WoW character's cached completed achievements (newest first).
func (h *handlers) wowCharacterAchievements(c *fiber.Ctx) error {
	data, err := h.store.WowCharacterAchievements(c.Context(), c.Params("id"), c.Params("realm"), c.Params("name"))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return c.JSON(fiber.Map{"achievements": []any{}})
	}
	return c.JSON(fiber.Map{"achievements": json.RawMessage(data)})
}
