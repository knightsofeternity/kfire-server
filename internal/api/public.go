// Package api — public.go wires /api/public/v1, the read-only API-key surface.
// Handlers evaluate privacy as a non-privileged viewer (viewer id "", role
// "member"), so members' activity_visible / sessions_visible toggles apply.
package api

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// publicViewerRole is the role used for all key-authenticated reads: never
// self, never admin, so privacy toggles are honoured.
const publicViewerRole = "member"

// inviteURL builds the public registration link for an invite code. The shape
// must match what the SPA reads from the `?invite=` query param; it is shared
// by the admin and key-authenticated invite paths.
func inviteURL(publicURL, code string) string {
	return publicURL + "/?invite=" + code
}

// POST /api/public/v1/invites — create a registration invite (requires a key
// with can_invite). Role is always "member"; admin invites are not issuable over
// the public API. Returns 201 {code, url, expires_at}.
func (h *handlers) publicCreateInvite(c *fiber.Ctx) error {
	if !apiKeyCanInvite(c) {
		return errorJSON(c, fiber.StatusForbidden, "forbidden",
			"this API key cannot create invites")
	}

	// Body is optional and tolerated; role is forced to member regardless.
	var req struct {
		Role string `json:"role"`
	}
	_ = c.BodyParser(&req)

	code, err := newInviteCode()
	if err != nil {
		return err
	}
	var createdBy *string // public path has no user identity → created_by NULL
	expiresAt := time.Now().Add(inviteTTL)
	if err := h.store.CreateInvite(c.Context(), code, "via API key", publicViewerRole,
		createdBy, expiresAt); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"code":       code,
		"url":        inviteURL(h.cfg.PublicURL, code),
		"expires_at": expiresAt.UTC(),
	})
}

// GET /api/public/v1/presence
func (h *handlers) publicPresence(c *fiber.Ctx) error {
	rows, err := h.store.ListPresence(c.Context())
	if err != nil {
		return err
	}
	entries := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		online := h.hub.OnlineSince(r.UserID)
		showGame := r.ActivityVisible // public viewer: only when the member opted in
		entries = append(entries, h.presenceEntry(r.UserID, r.Username, r.AvatarURL, r.Game, r.StartedAt, online, showGame))
	}
	return c.JSON(fiber.Map{"entries": entries})
}

// GET /api/public/v1/members — roster + linked-account summary.
func (h *handlers) publicMembers(c *fiber.Ctx) error {
	users, err := h.store.ListUsers(c.Context())
	if err != nil {
		return err
	}
	out := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		if u.BannedAt != nil {
			continue // banned members are not exposed publicly
		}
		m := fiber.Map{"id": u.ID, "username": u.Username}
		if u.AvatarURL != nil {
			m["avatar_url"] = *u.AvatarURL
		}
		if linked, err := h.store.ListLinkedAccounts(c.Context(), u.ID); err == nil {
			conns := make([]fiber.Map, len(linked))
			for i, a := range linked {
				conns[i] = connectionJSON(a)
			}
			m["connections"] = conns
		}
		out = append(out, m)
	}
	return c.JSON(fiber.Map{"members": out})
}

// GET /api/public/v1/members/:id — profile, gated by the member's privacy toggles.
func (h *handlers) publicMemberDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	u, err := h.store.GetUserByID(c.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && u.BannedAt != nil) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "member not found")
	}
	if err != nil {
		return err
	}

	// Public viewer: never self, never admin.
	vis := sessionVisibilityFor("", publicViewerRole, id, u.ActivityVisible, u.SessionsVisible)
	presence := h.userPresence(c, u, u.ActivityVisible)

	resp := fiber.Map{
		"user":     fiber.Map{"id": u.ID, "username": u.Username},
		"presence": presence,
	}
	if u.AvatarURL != nil {
		resp["user"].(fiber.Map)["avatar_url"] = *u.AvatarURL
	}

	if linked, err := h.store.ListLinkedAccounts(c.Context(), id); err == nil {
		conns := make([]fiber.Map, len(linked))
		for i, a := range linked {
			conns[i] = connectionJSON(a)
		}
		resp["connections"] = conns
	}

	// Playtime stats are "sessions" data: omit entirely when the member hid them.
	if !vis.HideAll {
		if stats, err := h.store.UserGameStats(c.Context(), id); err == nil {
			gs := make([]fiber.Map, len(stats))
			var total int64
			for i, st := range stats {
				gs[i] = fiber.Map{
					"game":           h.gameJSON(st.Game),
					"total_seconds":  st.TotalSeconds,
					"session_count":  st.SessionCount,
					"last_played_at": st.LastPlayedAt.UTC(),
				}
				total += st.TotalSeconds
			}
			resp["game_stats"] = gs
			resp["total_seconds"] = total
		}
	}
	return c.JSON(resp)
}

// GET /api/public/v1/members/:id/games — owned/played library.
func (h *handlers) publicMemberGames(c *fiber.Ctx) error {
	id := c.Params("id")
	u, err := h.store.GetUserByID(c.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && u.BannedAt != nil) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "member not found")
	}
	if err != nil {
		return err
	}
	// Library is sessions/ownership data: empty when the member hid sessions.
	if sessionVisibilityFor("", publicViewerRole, id, u.ActivityVisible, u.SessionsVisible).HideAll {
		return c.JSON(fiber.Map{"games": []fiber.Map{}})
	}
	owned, err := h.store.OwnedGames(c.Context(), id)
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(owned))
	for i, og := range owned {
		out[i] = fiber.Map{"game": h.gameJSON(og.Game), "source": og.Source}
	}
	return c.JSON(fiber.Map{"games": out})
}

// GET /api/public/v1/members/:id/games/:slug — one member's detail for one game.
// Reads only cached data; never triggers a Battle.net refresh.
func (h *handlers) publicMemberGameDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	u, err := h.store.GetUserByID(c.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && u.BannedAt != nil) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "member not found")
	}
	if err != nil {
		return err
	}
	g, err := h.store.GetGameBySlug(c.Context(), c.Params("slug"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "game not found")
	}
	if err != nil {
		return err
	}
	hideSessions := sessionVisibilityFor("", publicViewerRole, id, u.ActivityVisible, u.SessionsVisible).HideAll

	resp := fiber.Map{"game": h.gameJSON(g)}

	if !hideSessions {
		if stats, err := h.store.UserGameStats(c.Context(), id); err == nil {
			for _, st := range stats {
				if st.Game.ID == g.ID {
					resp["total_seconds"] = st.TotalSeconds
					resp["session_count"] = st.SessionCount
					resp["last_played_at"] = st.LastPlayedAt.UTC()
					break
				}
			}
		}
	}

	// WoW characters (account/profile data, not session activity) — always shown.
	if chars, err := h.store.WowCharactersForUserGame(c.Context(), id, g.ID); err == nil && len(chars) > 0 {
		cards := make([]fiber.Map, len(chars))
		for i, ch := range chars {
			m := fiber.Map{
				"name":               ch.Name,
				"realm":              ch.RealmName,
				"realm_slug":         ch.RealmSlug,
				"class":              ch.Class,
				"race":               ch.Race,
				"faction":            ch.Faction,
				"level":              ch.Level,
				"item_level":         ch.ItemLevel,
				"achievement_points": ch.AchievementPoints,
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

	if data, err := h.store.GameProfileForUserGame(c.Context(), id, g.ID); err == nil && len(data) > 0 {
		resp["bnet_profile"] = json.RawMessage(data)
	}
	return c.JSON(resp)
}
