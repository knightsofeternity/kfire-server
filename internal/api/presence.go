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
		hasOpen := r.Game != nil
		status := store.PresenceStatus(hasOpen, hasOpen && showGame, online != nil)

		entry := fiber.Map{"user_id": r.UserID, "username": r.Username, "status": status, "game": nil}
		if r.AvatarURL != nil {
			entry["avatar_url"] = *r.AvatarURL
		}
		if status == "in_game" {
			entry["game"] = h.gameJSON(*r.Game)
			if r.StartedAt != nil {
				entry["since"] = r.StartedAt.UTC()
			}
		} else if status == "online" && online != nil {
			entry["since"] = online.UTC()
		}
		entries = append(entries, entry)
	}

	return c.JSON(fiber.Map{"entries": entries})
}

// userPresence builds a single presence entry for a profile page.
func (h *handlers) userPresence(c *fiber.Ctx, u store.User, showGame bool) fiber.Map {
	online := h.hub.OnlineSince(u.ID)
	var sess *store.Session
	if s, err := h.store.LatestOpenSession(c.Context(), u.ID); err == nil {
		sess = s
	}
	status := store.PresenceStatus(sess != nil, sess != nil && showGame, online != nil)

	entry := fiber.Map{"user_id": u.ID, "username": u.Username, "status": status, "game": nil}
	if u.AvatarURL != nil {
		entry["avatar_url"] = *u.AvatarURL
	}
	if status == "in_game" && sess != nil {
		entry["game"] = h.gameJSON(sess.Game)
		entry["since"] = sess.StartedAt.UTC()
	} else if status == "online" && online != nil {
		entry["since"] = online.UTC()
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

// gameJSON renders a game with image URLs pointing at our lazy image cache
// (/img/games/:id/:kind) rather than directly at Discord's CDN.
func (h *handlers) gameJSON(g store.Game) fiber.Map {
	m := fiber.Map{"id": g.ID, "name": g.Name, "slug": g.Slug}
	if g.IconURL != nil {
		m["icon_url"] = h.cfg.PublicURL + "/img/games/" + g.ID + "/icon"
	}
	if g.CoverURL != nil {
		m["cover_url"] = h.cfg.PublicURL + "/img/games/" + g.ID + "/cover"
	}
	return m
}

// GET /api/v1/sessions  (authenticated)
// sessionVisibility decides how much of a target user's sessions a viewer sees.
type sessionVisibility struct {
	HideAll  bool // hide the entire recent-sessions list
	HideOpen bool // hide only in-progress (live) sessions
}

// sessionVisibilityFor applies a member's privacy toggles for a given viewer.
// Self and admins always see everything.
func sessionVisibilityFor(viewerID, viewerRole, targetID string, activityVisible, sessionsVisible bool) sessionVisibility {
	if targetID == "" || targetID == viewerID || viewerRole == "admin" {
		return sessionVisibility{}
	}
	return sessionVisibility{HideAll: !sessionsVisible, HideOpen: !activityVisible}
}

func (h *handlers) sessions(c *fiber.Ctx) error {
	filter := store.SessionFilter{
		UserID: c.Query("user_id"),
		GameID: c.Query("game_id"),
		Limit:  c.QueryInt("limit", 25),
		Cursor: c.Query("cursor"),
	}

	// Honor a target member's privacy toggles. Self and admins see everything;
	// another member who hid their activity has in-progress sessions dropped,
	// and one who hid their recent sessions gets an empty list.
	claims := mustClaims(c)
	if filter.UserID != "" && filter.UserID != claims.UserID && claims.Role != "admin" {
		if target, err := h.store.GetUserByID(c.Context(), filter.UserID); err == nil {
			vis := sessionVisibilityFor(claims.UserID, claims.Role, filter.UserID,
				target.ActivityVisible, target.SessionsVisible)
			if vis.HideAll {
				return c.JSON(fiber.Map{"sessions": []fiber.Map{}})
			}
			filter.HideOpen = vis.HideOpen
		}
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
			"game":       h.gameJSON(s.Game),
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
