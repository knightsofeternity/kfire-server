package api

import (
	"errors"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/wowplayed"
)

// forever returns the WoW Forever game, or writes a 404.
func (h *handlers) forever(c *fiber.Ctx) (*store.Game, error) {
	g, err := h.store.GetGameBySlug(c.Context(), wowplayed.GuildSlug)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errorJSON(c, fiber.StatusNotFound, "not_found", "game not found")
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// GET /api/v1/wow/guilds  (authenticated)
// The WoW Forever guilds the KFireGuilde addon has reported.
func (h *handlers) wowGuilds(c *fiber.Ctx) error {
	g, err := h.forever(c)
	if g == nil {
		return err
	}
	guilds, err := h.store.WowGuilds(c.Context(), g.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"guilds": guilds})
}

type wowRecipe struct {
	ID       string   `json:"id"`
	Name     string   `json:"name,omitempty"`
	Crafters []string `json:"crafters"`
}

type wowProfession struct {
	Profession string                  `json:"profession"`
	Crafters   []store.WowGuildCrafter `json:"crafters"`
	Recipes    []wowRecipe             `json:"recipes"`
}

// GET /api/v1/wow/guilds/:id  (authenticated)
// One guild's page: roster merged with addon profiles and KFIRE members,
// professions with who knows each recipe, and the typical week of presence.
func (h *handlers) wowGuild(c *fiber.Ctx) error {
	game, err := h.forever(c)
	if game == nil {
		return err
	}
	ctx := c.Context()
	guild, err := h.store.WowGuildByID(ctx, game.ID, c.Params("id"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "guild not found")
	}
	if err != nil {
		return err
	}
	roster, err := h.store.WowGuildRoster(ctx, guild.ID)
	if err != nil {
		return err
	}
	profRows, err := h.store.WowGuildProfessionRows(ctx, guild.ID)
	if err != nil {
		return err
	}
	var ids []string
	seen := map[string]bool{}
	for _, crafters := range profRows {
		for _, cr := range crafters {
			for _, id := range cr.Recipes {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	names, err := h.store.WowItemNames(ctx, game.ID, ids)
	if err != nil {
		return err
	}
	professions := make([]wowProfession, 0, len(profRows))
	for prof, crafters := range profRows {
		byID := map[string]*wowRecipe{}
		for _, cr := range crafters {
			for _, id := range cr.Recipes {
				r := byID[id]
				if r == nil {
					r = &wowRecipe{ID: id, Name: names[id]}
					byID[id] = r
				}
				r.Crafters = append(r.Crafters, cr.Name)
			}
		}
		recipes := make([]wowRecipe, 0, len(byID))
		for _, r := range byID {
			recipes = append(recipes, *r)
		}
		// Named recipes first, alphabetically; unnamed ones after, by id.
		sort.Slice(recipes, func(i, j int) bool {
			a, b := recipes[i], recipes[j]
			if (a.Name == "") != (b.Name == "") {
				return a.Name != ""
			}
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.ID < b.ID
		})
		professions = append(professions, wowProfession{Profession: prof, Crafters: crafters, Recipes: recipes})
	}
	sort.Slice(professions, func(i, j int) bool { return professions[i].Profession < professions[j].Profession })

	days, err := h.store.WowGuildPresenceSince(ctx, guild.ID, wowplayed.PresenceSince(time.Now()))
	if err != nil {
		return err
	}
	week, best := wowplayed.TypicalWeek(days, 5)
	return c.JSON(fiber.Map{
		"guild":       guild,
		"roster":      roster,
		"professions": professions,
		"presence": fiber.Map{
			"weeks": wowplayed.PresenceWeeks,
			"days":  len(days),
			"week":  week,
			"best":  best,
		},
	})
}
