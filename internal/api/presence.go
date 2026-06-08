package api

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

// GET /api/v1/presence  (authenticated)
//
// Snapshot for initial page load; live updates come from the WebSocket.
// Members who disabled activity visibility appear capped at "online" to
// everyone but themselves and admins.
func (h *handlers) presence(c *fiber.Ctx) error {
	rows, err := h.store.ListPresence(c.Context())
	if err != nil {
		return err
	}

	claims := mustClaims(c)
	entries := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		online := h.hub.OnlineSince(r.UserID)
		showGame := r.ActivityVisible || r.UserID == claims.UserID || claims.Role == "admin"

		entry := fiber.Map{
			"user_id":  r.UserID,
			"username": r.Username,
			"status":   "offline",
			"game":     nil,
		}
		if r.AvatarURL != nil {
			entry["avatar_url"] = *r.AvatarURL
		}
		if online != nil {
			entry["status"] = "online"
			entry["since"] = online.UTC()
			if r.Game != nil && showGame {
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

// userPresence builds a single presence entry for a profile page.
func (h *handlers) userPresence(c *fiber.Ctx, u store.User, showGame bool) fiber.Map {
	entry := fiber.Map{
		"user_id":  u.ID,
		"username": u.Username,
		"status":   "offline",
		"game":     nil,
	}
	if u.AvatarURL != nil {
		entry["avatar_url"] = *u.AvatarURL
	}

	since := h.hub.OnlineSince(u.ID)
	if since == nil {
		return entry
	}
	entry["status"] = "online"
	entry["since"] = since.UTC()

	if showGame {
		if sess, err := h.store.LatestOpenSession(c.Context(), u.ID); err == nil && sess != nil {
			entry["status"] = "in_game"
			entry["game"] = presenceGameJSON(sess.Game)
			entry["since"] = sess.StartedAt.UTC()
		}
	}
	return entry
}

// presenceUser converts a store.User to the hub's broadcast input.
func presenceUser(u store.User) ws.PresenceUser {
	return ws.PresenceUser{
		ID:              u.ID,
		Username:        u.Username,
		AvatarURL:       u.AvatarURL,
		ActivityVisible: u.ActivityVisible,
	}
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
