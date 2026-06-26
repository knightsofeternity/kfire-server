package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// GET /api/v1/users/:id/games/:slug  (authenticated)
// One member's detail for one game: playtime, achievements, and any
// plugin-provided game-specific blocks when present.
func (h *handlers) userGameDetail(c *fiber.Ctx) error {
	userID := c.Params("id")
	g, err := h.store.GetGameBySlug(c.Context(), c.Params("slug"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "game not found")
	}
	if err != nil {
		return err
	}

	resp := fiber.Map{"game": h.gameJSON(g)}

	// Playtime for this game (from the member's stats; 0 if none/unplayed).
	if stats, err := h.store.UserGameStats(c.Context(), userID); err == nil {
		for _, st := range stats {
			if st.Game.ID == g.ID {
				resp["total_seconds"] = st.TotalSeconds
				resp["session_count"] = st.SessionCount
				resp["last_played_at"] = st.LastPlayedAt.UTC()
				break
			}
		}
	}

	// Achievements the member unlocked in this game.
	if achs, err := h.store.ListUserAchievements(c.Context(), userID, g.ID, 60, 0); err == nil {
		out := make([]fiber.Map, len(achs))
		for i, a := range achs {
			m := fiber.Map{"api_name": a.APIName, "unlocked_at": a.UnlockedAt.UTC()}
			if a.DisplayName != nil {
				m["display_name"] = *a.DisplayName
			}
			if a.IconURL != nil {
				m["icon_url"] = *a.IconURL
			}
			out[i] = m
		}
		resp["achievements"] = out
	}

	for _, p := range h.plugins.ForSlug(g.Slug) {
		p.Refresh(c.Context(), userID, g.Slug)
		block, err := p.UserGameDetail(c.Context(), userID, g)
		if err != nil {
			slog.Warn("userGameDetail plugin", "plugin", p.ID(), "err", err)
			continue
		}
		for k, v := range block {
			resp[k] = v
		}
	}

	return c.JSON(resp)
}
