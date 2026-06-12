package api

import "github.com/gofiber/fiber/v2"

// GET /api/v1/users/:id/games  (authenticated)
// The member's owned library: full Steam library + Battle.net-inferred games.
func (h *handlers) userGames(c *fiber.Ctx) error {
	owned, err := h.store.OwnedGames(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(owned))
	for i, og := range owned {
		out[i] = fiber.Map{"game": h.gameJSON(og.Game), "source": og.Source}
	}
	return c.JSON(fiber.Map{"games": out})
}
