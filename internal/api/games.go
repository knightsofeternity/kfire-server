package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"time"

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

	var wowCards []fiber.Map
	var wowSynced time.Time
	if g.Slug == "world-of-warcraft" || g.Slug == "world-of-warcraft-classic" {
		h.bnetSync.RefreshWoW(c.Context(), mustClaims(c).UserID, g.Slug)
		wowChars, synced, err := h.store.WowCharactersByGame(c.Context(), g.ID)
		if err != nil {
			return err
		}
		wowSynced = synced
		wowCards = make([]fiber.Map, len(wowChars))
		for i, ch := range wowChars {
			m := fiber.Map{
				"user_id": ch.UserID, "name": ch.Name, "realm": ch.RealmName,
				"class": ch.Class, "race": ch.Race, "faction": ch.Faction,
				"level": ch.Level, "item_level": ch.ItemLevel,
				"achievement_points": ch.AchievementPoints,
			}
			if ch.MythicRating != nil {
				m["mythic_rating"] = *ch.MythicRating
			}
			if len(ch.RaidSummary) > 0 {
				m["raid_summary"] = json.RawMessage(ch.RaidSummary)
			}
			wowCards[i] = m
		}
	}

	var bnetProfiles []fiber.Map
	var bnetSynced time.Time
	if g.Slug == "diablo-iii" || g.Slug == "starcraft-ii-battle-chest" {
		h.bnetSync.RefreshBnetGame(c.Context(), mustClaims(c).UserID, g.Slug)
		profs, synced, err := h.store.GameProfilesByGame(c.Context(), g.ID)
		if err != nil {
			return err
		}
		bnetSynced = synced
		bnetProfiles = make([]fiber.Map, len(profs))
		for i, p := range profs {
			bnetProfiles[i] = fiber.Map{
				"user_id":  p.UserID,
				"username": p.Username,
				"data":     json.RawMessage(p.Data),
			}
		}
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

	achievements, err := h.store.GameAchievements(c.Context(), g.ID, 60)
	if err != nil {
		return err
	}
	achs := make([]fiber.Map, len(achievements))
	for i, a := range achievements {
		m := fiber.Map{"api_name": a.APIName, "unlocks": a.Unlocks}
		if a.DisplayName != nil {
			m["display_name"] = *a.DisplayName
		}
		if a.IconURL != nil {
			m["icon_url"] = *a.IconURL
		}
		achs[i] = m
	}

	resp := fiber.Map{
		"game":          h.gameJSON(g),
		"total_seconds": totalSeconds,
		"player_count":  players,
		"leaderboard":   board,
		"achievements":  achs,
	}
	if g.Slug == "world-of-warcraft" || g.Slug == "world-of-warcraft-classic" {
		resp["wow_characters"] = wowCards
		resp["wow_synced_at"] = wowSynced
	}
	if g.Slug == "diablo-iii" || g.Slug == "starcraft-ii-battle-chest" {
		resp["bnet_profiles"] = bnetProfiles
		resp["bnet_synced_at"] = bnetSynced
	}
	return c.JSON(resp)
}

// GET /api/v1/achievements?user_id=&game_id=&limit=&offset=  (authenticated)
//
// A member's unlocked achievements, most recent first, optionally filtered by
// game and paginated, plus the per-game breakdown for the filter UI.
func (h *handlers) userAchievements(c *fiber.Ctx) error {
	userID := c.Query("user_id")
	if userID == "" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "user_id is required")
	}
	limit := c.QueryInt("limit", 24)
	offset := c.QueryInt("offset", 0)

	list, err := h.store.ListUserAchievements(c.Context(), userID, c.Query("game_id"), limit, offset)
	if err != nil {
		return err
	}
	achs := make([]fiber.Map, len(list))
	for i, a := range list {
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
		achs[i] = m
	}

	games, err := h.store.UserAchievementGames(c.Context(), userID)
	if err != nil {
		return err
	}
	gameOpts := make([]fiber.Map, len(games))
	for i, ag := range games {
		gameOpts[i] = fiber.Map{"game": h.gameJSON(ag.Game), "count": ag.Count}
	}

	return c.JSON(fiber.Map{
		"achievements": achs,
		"games":        gameOpts,
		"has_more":     len(list) == limit,
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
