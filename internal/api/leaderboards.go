package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	leaderboardWindowDays = 7
	leaderboardLimit      = 10
)

// weeklyLeaderboardsJSON builds the shared response body for both API surfaces.
// The privacy rules live entirely in the store query, so the internal and
// public handlers are identical in output.
func (h *handlers) weeklyLeaderboardsJSON(c *fiber.Ctx) (fiber.Map, error) {
	res, err := h.store.WeeklyLeaderboards(c.Context(), leaderboardWindowDays, leaderboardLimit)
	if err != nil {
		return nil, err
	}
	players := make([]fiber.Map, len(res.TopPlayers))
	for i, p := range res.TopPlayers {
		m := fiber.Map{"user_id": p.UserID, "username": p.Username, "total_seconds": p.TotalSeconds}
		if p.AvatarURL != nil {
			m["avatar_url"] = *p.AvatarURL
		}
		players[i] = m
	}
	games := make([]fiber.Map, len(res.TopGames))
	for i, lg := range res.TopGames {
		games[i] = fiber.Map{
			"game":          h.gameJSON(lg.Game),
			"total_seconds": lg.TotalSeconds,
			"player_count":  lg.PlayerCount,
		}
	}
	return fiber.Map{
		"window_days":  res.WindowDays,
		"generated_at": time.Now().UTC(),
		"top_players":  players,
		"top_games":    games,
	}, nil
}

// GET /api/v1/leaderboards/weekly (authenticated SPA viewer).
func (h *handlers) weeklyLeaderboards(c *fiber.Ctx) error {
	out, err := h.weeklyLeaderboardsJSON(c)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// GET /api/public/v1/leaderboards/weekly (API key).
func (h *handlers) publicWeeklyLeaderboards(c *fiber.Ctx) error {
	out, err := h.weeklyLeaderboardsJSON(c)
	if err != nil {
		return err
	}
	return c.JSON(out)
}
