package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// GET /api/v1/users/me  (authenticated)
func (h *handlers) me(c *fiber.Ctx) error {
	u, err := h.store.GetUserByID(c.Context(), mustClaims(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(userJSON(u, true))
}

// PATCH /api/v1/users/me  (authenticated)
//
// Updates the owner's account settings. Currently the activity privacy toggle.
func (h *handlers) updateMe(c *fiber.Ctx) error {
	var req struct {
		ActivityVisible *bool   `json:"activity_visible"`
		SessionsVisible *bool   `json:"sessions_visible"`
		PresenceStatus  *string `json:"presence_status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}

	claims := mustClaims(c)
	if req.PresenceStatus != nil {
		switch *req.PresenceStatus {
		case "online", "invisible", "offline":
		default:
			return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
				"presence_status must be one of online, invisible, offline")
		}
		if err := h.store.SetPresenceStatus(c.Context(), claims.UserID, *req.PresenceStatus); err != nil {
			return err
		}
		// A chosen-status change must reflect immediately in live presence.
		u, err := h.store.GetUserByID(c.Context(), claims.UserID)
		if err == nil {
			h.hub.BroadcastPresence(c.Context(), presenceUser(u))
		}
	}
	if req.ActivityVisible != nil {
		if err := h.store.SetActivityVisible(c.Context(), claims.UserID, *req.ActivityVisible); err != nil {
			return err
		}
		// A visibility change must reflect immediately in live presence.
		u, err := h.store.GetUserByID(c.Context(), claims.UserID)
		if err == nil {
			h.hub.BroadcastPresence(c.Context(), presenceUser(u))
		}
	}
	if req.SessionsVisible != nil {
		// Read on profile fetch only, so no live presence broadcast is needed.
		if err := h.store.SetSessionsVisible(c.Context(), claims.UserID, *req.SessionsVisible); err != nil {
			return err
		}
	}

	u, err := h.store.GetUserByID(c.Context(), claims.UserID)
	if err != nil {
		return err
	}
	return c.JSON(userJSON(u, true))
}

// GET /api/v1/users/:id  (authenticated)
//
// Public profile: identity, current presence, and per-game playtime stats.
func (h *handlers) userProfile(c *fiber.Ctx) error {
	id := c.Params("id")
	u, err := h.store.GetUserByID(c.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "user not found")
	}
	if err != nil {
		return err
	}

	// When viewing your OWN profile, warm your game data in the background so it
	// surfaces in your library and game cards. Driven by the ACTIVE plugins, so a
	// disabled plugin performs no crawl (and a future connector is covered
	// automatically). Self-only (no fetching other members' data, no thundering
	// herd); each plugin's Refresh no-ops when not linked / scope missing /
	// throttled, and runs on a detached context so it outlives the request.
	if mustClaims(c).UserID == id {
		// c.Params returns a string backed by Fiber's per-request buffer, which
		// is recycled once the handler returns; clone it so the detached
		// goroutine doesn't read a value overwritten by a later request.
		uid := strings.Clone(id)
		active := h.plugins.ActivePlugins()
		if len(active) > 0 {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				for _, p := range active {
					for _, slug := range p.Slugs() {
						p.Refresh(ctx, uid, slug)
					}
				}
			}()
		}
	}

	stats, err := h.store.UserGameStats(c.Context(), id)
	if err != nil {
		return err
	}

	gameStats := make([]fiber.Map, len(stats))
	var totalSeconds int64
	for i, st := range stats {
		gameStats[i] = fiber.Map{
			"game":           h.gameJSON(st.Game),
			"total_seconds":  st.TotalSeconds,
			"session_count":  st.SessionCount,
			"last_played_at": st.LastPlayedAt.UTC(),
		}
		totalSeconds += st.TotalSeconds
	}

	// Self and admins always see the real live status; the privacy toggle
	// only hides it from other members.
	isSelf := mustClaims(c).UserID == id
	canSeeActivity := isSelf || mustClaims(c).Role == "admin" || u.ActivityVisible
	presence := h.userPresence(c, u, canSeeActivity)

	linked, err := h.store.ListLinkedAccounts(c.Context(), id)
	if err != nil {
		return err
	}
	connections := make([]fiber.Map, len(linked))
	for i, a := range linked {
		connections[i] = connectionJSON(a)
	}

	achievements, err := h.store.RecentAchievements(c.Context(), id, 24)
	if err != nil {
		return err
	}
	achievementCount, err := h.store.AchievementCount(c.Context(), id)
	if err != nil {
		return err
	}
	recentAchievements := make([]fiber.Map, len(achievements))
	for i, a := range achievements {
		m := fiber.Map{
			"game":        h.gameJSON(a.Game),
			"api_name":    a.APIName,
			"unlocked_at": a.UnlockedAt.UTC(),
		}
		if a.DisplayName != nil {
			m["display_name"] = *a.DisplayName
		}
		if a.IconURL != nil {
			m["icon_url"] = *a.IconURL
		}
		recentAchievements[i] = m
	}

	return c.JSON(fiber.Map{
		"user":                userJSON(u, isSelf),
		"presence":            presence,
		"total_seconds":       totalSeconds,
		"game_stats":          gameStats,
		"connections":         connections,
		"achievement_count":   achievementCount,
		"recent_achievements": recentAchievements,
	})
}
