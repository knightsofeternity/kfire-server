// Package api — public.go wires /api/public/v1, the read-only API-key surface.
// Handlers evaluate privacy as a non-privileged viewer (viewer id "", role
// "member"), so members' activity_visible / sessions_visible toggles apply.
package api

import (
	"errors"
	"log/slog"
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

// POST /api/public/v1/invites - create a registration invite (requires a key
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
		entries = append(entries, h.presenceEntry(r.UserID, r.Username, r.AvatarURL, r.Game, r.StartedAt, online, showGame, r.PresenceStatus))
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
// Reads only cached data; never triggers a connector refresh.
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

	// Plugin data (WoW characters, Diablo III / StarCraft II profiles) — reads
	// only cached data; never triggers a Battle.net refresh (public endpoint).
	for _, p := range h.plugins.ForSlug(g.Slug) {
		block, err := p.UserGameDetail(c.Context(), id, g)
		if err != nil {
			slog.Warn("publicMemberGameDetail plugin", "plugin", p.ID(), "err", err)
			continue
		}
		for k, v := range block {
			resp[k] = v
		}
	}
	return c.JSON(resp)
}

// GET /api/public/v1/games/:slug — one game's aggregate: who played it over the
// last 7 days (recent_players) and the all-time leaderboard (all_time_players).
// Both honor member privacy (banned, sessions_visible) and hidden games.
func (h *handlers) publicGameDetail(c *fiber.Ctx) error {
	g, err := h.store.GetGameBySlug(c.Context(), c.Params("slug"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "game not found")
	}
	if err != nil {
		return err
	}
	if g.Hidden {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "game not found")
	}

	recentEntries, err := h.store.GameRecentPlayers(c.Context(), g.ID, 7, 25)
	if err != nil {
		return err
	}
	recent := make([]fiber.Map, len(recentEntries))
	for i, e := range recentEntries {
		m := fiber.Map{"user_id": e.UserID, "username": e.Username, "total_seconds": e.TotalSeconds}
		if e.AvatarURL != nil {
			m["avatar_url"] = *e.AvatarURL
		}
		recent[i] = m
	}

	entries, totalSeconds, players, err := h.store.GameLeaderboard(c.Context(), g.ID, 25, true)
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
		"game":             h.gameJSON(g),
		"window_days":      7,
		"total_seconds":    totalSeconds,
		"player_count":     players,
		"recent_players":   recent,
		"all_time_players": board,
	})
}
