package api

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// GET /api/v1/users/:id/games/:slug  (authenticated)
// One member's detail for one game: playtime, achievements, and Battle.net data
// (WoW characters / Diablo III / StarCraft II) when present.
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

	// WoW characters for this member+game.
	if chars, err := h.store.WowCharactersForUserGame(c.Context(), userID, g.ID); err == nil && len(chars) > 0 {
		cards := make([]fiber.Map, len(chars))
		for i, ch := range chars {
			m := fiber.Map{
				"name":       ch.Name,
				"realm":      ch.RealmName,
				"class":      ch.Class,
				"race":       ch.Race,
				"faction":    ch.Faction,
				"level":      ch.Level,
				"item_level": ch.ItemLevel,
			}
			if ch.MythicRating != nil {
				m["mythic_rating"] = *ch.MythicRating
			}
			if len(ch.RaidSummary) > 0 {
				m["raid_summary"] = json.RawMessage(ch.RaidSummary)
			}
			cards[i] = m
		}
		resp["wow_characters"] = cards
	}

	// Battle.net profile blob (Diablo III / StarCraft II) for this member+game.
	if data, err := h.store.GameProfileForUserGame(c.Context(), userID, g.ID); err == nil && len(data) > 0 {
		resp["bnet_profile"] = json.RawMessage(data)
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

	return c.JSON(resp)
}
