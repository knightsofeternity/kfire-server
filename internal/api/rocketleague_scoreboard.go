package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/knightsofeternity/kfire-server/internal/rocketleague"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// scoreboardJSON renders a scoreboard. An anonymous line carries no name field
// at all, not even empty: absence is what the page tests.
func scoreboardJSON(matchID string, sb rocketleague.Scoreboard) map[string]any {
	teams := make([]map[string]any, 0, 2)
	for _, t := range sb.Teams {
		rows := make([]map[string]any, 0, len(t.Rows))
		for _, r := range t.Rows {
			row := map[string]any{
				"score": r.Player.Score, "goals": r.Player.Goals,
				"assists": r.Player.Assists, "saves": r.Player.Saves,
				"shots": r.Player.Shots, "left": r.Player.Left, "mvp": r.MVP,
			}
			if r.UserID != "" {
				row["user_id"] = r.UserID
				row["username"] = r.Username
				if r.AvatarURL != nil {
					row["avatar_url"] = *r.AvatarURL
				}
			} else {
				row["anon_index"] = r.AnonIndex
			}
			rows = append(rows, row)
		}
		teams = append(teams, map[string]any{
			"team": t.Team, "score": t.Score, "winner": t.Winner, "rows": rows,
		})
	}
	return map[string]any{
		"match_id":       matchID,
		"reference_team": sb.ReferenceTeam,
		"teams":          teams,
	}
}

// GET /api/v1/rocket-league/matches/:id/scoreboard  (authenticated)
//
// Every refusal is the same 404, so the answer never tells a viewer that a
// hidden member played a match: unknown id, banned author, author hidden from
// this viewer, match without a scoreboard, plugin switched off.
func (h *handlers) rocketLeagueScoreboard(c *fiber.Ctx) error {
	notFound := func() error {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "scoreboard not found")
	}
	id := c.Params("id")
	if _, err := uuid.Parse(id); err != nil {
		return notFound()
	}
	if len(h.plugins.ForSlug(rocketleague.SLUG)) == 0 {
		return notFound()
	}
	claims := mustClaims(c)
	viewer := store.RecapViewer{UserID: claims.UserID, IsAdmin: claims.Role == "admin"}

	ref, err := h.store.RocketLeagueReportByID(c.Context(), id, viewer)
	if errors.Is(err, store.ErrNotFound) {
		return notFound()
	}
	if err != nil {
		return err
	}
	if !ref.Visible || ref.MatchKey == nil {
		return notFound()
	}
	others, err := h.store.RocketLeagueMatchPlayers(c.Context(), id)
	if err != nil {
		return err
	}
	peers, err := h.store.RocketLeagueReportsByKey(c.Context(), *ref.MatchKey, id, viewer)
	if err != nil {
		return err
	}
	return c.JSON(scoreboardJSON(id, rocketleague.BuildScoreboard(ref, others, peers)))
}
