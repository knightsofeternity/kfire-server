package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/games"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func gameJSON(g store.Game) fiber.Map {
	m := fiber.Map{
		"id":               g.ID,
		"name":             g.Name,
		"slug":             g.Slug,
		"executable_names": g.ExecutableNames,
		"platform":         g.Platform,
	}
	if g.IconURL != nil {
		m["icon_url"] = *g.IconURL
	}
	return m
}

// GET /api/v1/games  (authenticated)
//
// Returns the full catalog; the desktop client downloads it to match local
// processes and caches it.
func (h *handlers) listGames(c *fiber.Ctx) error {
	list, err := h.store.ListGames(c.Context())
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(list))
	for i, g := range list {
		out[i] = gameJSON(g)
	}
	return c.JSON(fiber.Map{"games": out})
}

// POST /api/v1/admin/games/sync  (admin)
//
// Re-imports the Discord detectable-games catalog. Synchronous: the download
// is ~10 MB and the upsert a few seconds — acceptable for an admin action.
func (h *handlers) syncGames(c *fiber.Ctx) error {
	seeds, err := games.FetchSeed(c.Context())
	if err != nil {
		slog.Error("games sync: fetch", "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "upstream_unavailable",
			"could not download the games catalog")
	}
	n, err := h.store.UpsertGames(c.Context(), seeds)
	if err != nil {
		return err
	}
	slog.Info("games sync: done", "upserted", n)
	return c.JSON(fiber.Map{"upserted": n})
}
