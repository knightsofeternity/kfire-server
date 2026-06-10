package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/games"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func (h *handlers) gameCatalogJSON(g store.Game) fiber.Map {
	m := h.gameJSON(g)
	m["executable_names"] = g.ExecutableNames
	m["platform"] = g.Platform
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
		out[i] = h.gameCatalogJSON(g)
	}
	return c.JSON(fiber.Map{"games": out})
}

// GET /api/v1/games/played  (authenticated)
//
// Games actually played in the org, alphabetical, with the number of players
// and the cumulative time. Powers the games list page.
func (h *handlers) listPlayedGames(c *fiber.Ctx) error {
	list, err := h.store.ListPlayedGames(c.Context())
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(list))
	for i, gs := range list {
		m := h.gameJSON(gs.Game)
		m["player_count"] = gs.PlayerCount
		m["total_seconds"] = gs.TotalSeconds
		out[i] = m
	}
	return c.JSON(fiber.Map{"games": out})
}

// GET /api/v1/games/:slug  (authenticated)
//
// Game detail with the org leaderboard (top players by playtime).
func (h *handlers) gameDetail(c *fiber.Ctx) error {
	g, err := h.store.GetGameBySlug(c.Context(), c.Params("slug"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "game not found")
	}
	if err != nil {
		return err
	}

	entries, totalSeconds, players, err := h.store.GameLeaderboard(c.Context(), g.ID, 25)
	if err != nil {
		return err
	}
	board := make([]fiber.Map, len(entries))
	for i, e := range entries {
		m := fiber.Map{
			"user_id":       e.UserID,
			"username":      e.Username,
			"total_seconds": e.TotalSeconds,
			"session_count": e.SessionCount,
		}
		if e.AvatarURL != nil {
			m["avatar_url"] = *e.AvatarURL
		}
		board[i] = m
	}

	return c.JSON(fiber.Map{
		"game":          h.gameJSON(g),
		"total_seconds": totalSeconds,
		"player_count":  players,
		"leaderboard":   board,
	})
}

// POST /api/v1/admin/games/sync  (admin)
//
// Re-imports the Discord detectable-games catalog. Synchronous: the download
// is ~10 MB and the upsert a few seconds - acceptable for an admin action.
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
