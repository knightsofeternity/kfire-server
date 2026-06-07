package api

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// GET /api/v1/presence  (authenticated)
//
// Snapshot for initial page load; live updates come from the WebSocket.
func (h *handlers) presence(c *fiber.Ctx) error {
	rows, err := h.store.ListPresence(c.Context())
	if err != nil {
		return err
	}

	entries := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		entry := fiber.Map{
			"user_id":  r.UserID,
			"username": r.Username,
			"status":   "offline",
			"game":     nil,
		}

		if since := h.hub.OnlineSince(r.UserID); since != nil {
			entry["status"] = "online"
			entry["since"] = since.UTC()
			if r.Game != nil {
				entry["status"] = "in_game"
				entry["game"] = presenceGameJSON(*r.Game)
				if r.StartedAt != nil {
					entry["since"] = r.StartedAt.UTC()
				}
			}
		}
		entries = append(entries, entry)
	}

	return c.JSON(fiber.Map{"entries": entries})
}

func presenceGameJSON(g store.Game) fiber.Map {
	m := fiber.Map{"id": g.ID, "name": g.Name, "slug": g.Slug}
	if g.IconURL != nil {
		m["icon_url"] = *g.IconURL
	}
	return m
}

// GET /api/v1/sessions  (authenticated)
func (h *handlers) sessions(c *fiber.Ctx) error {
	filter := store.SessionFilter{
		UserID: c.Query("user_id"),
		GameID: c.Query("game_id"),
		Limit:  c.QueryInt("limit", 25),
		Cursor: c.Query("cursor"),
	}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
				"from must be RFC 3339")
		}
		filter.From = t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
				"to must be RFC 3339")
		}
		filter.To = t
	}

	sessions, next, err := h.store.ListSessions(c.Context(), filter)
	if err != nil {
		return err
	}

	out := make([]fiber.Map, len(sessions))
	for i, s := range sessions {
		m := fiber.Map{
			"id":         s.ID,
			"user_id":    s.UserID,
			"game":       presenceGameJSON(s.Game),
			"source":     s.Source,
			"started_at": s.StartedAt.UTC(),
			"ended_at":   nil,
		}
		if s.EndedAt != nil {
			m["ended_at"] = s.EndedAt.UTC()
		}
		if s.DurationSeconds != nil {
			m["duration_seconds"] = *s.DurationSeconds
		}
		out[i] = m
	}

	resp := fiber.Map{"sessions": out}
	if next != "" {
		resp["next_cursor"] = next
	}
	return c.JSON(resp)
}
