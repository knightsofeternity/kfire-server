// Package gameplugin defines the per-game plugin abstraction: each rich game
// integration (its crawl and its display contributions) behind one interface,
// plus a registry resolving which plugins are active.
package gameplugin

import (
	"context"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// Plugin is one game's rich integration. Implementations wrap a connector's
// game-specific crawl and shape the JSON blocks shown on game/player pages.
type Plugin interface {
	ID() string        // stable key: "wow", "d3", "sc2", "lol"
	Name() string      // admin-facing label: "World of Warcraft"
	Connector() string // credential dependency name, for the admin UI: "battlenet"
	Available() bool   // true when the underlying connector is configured
	Slugs() []string   // catalog slugs this plugin owns

	// Refresh crawls one member's data for gameSlug. Lazy + self-throttled.
	// Must no-op if gameSlug is not one of Slugs().
	Refresh(ctx context.Context, userID, gameSlug string)

	// GameDetail returns this plugin's aggregate block for the game page
	// (/games/:slug), e.g. {"wow_characters":[...], "wow_synced_at":...}, or nil.
	GameDetail(ctx context.Context, viewerID string, g store.Game) (map[string]any, error)

	// UserGameDetail returns one member's block for /users/:id/games/:slug, or nil.
	UserGameDetail(ctx context.Context, targetUserID string, g store.Game) (map[string]any, error)
}

// Info is the admin-facing description of a registered plugin.
type Info struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Connector string `json:"connector"`
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
}

// ActivePlugin is the public-config view of an active plugin.
type ActivePlugin struct {
	ID    string   `json:"id"`
	Slugs []string `json:"slugs"`
}
