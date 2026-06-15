// Package api wires the REST control plane.
//
// Contract reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/openapi.yaml
package api

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/knightsofeternity/kfire-server/internal/bnetsync"
	"github.com/knightsofeternity/kfire-server/internal/config"
	"github.com/knightsofeternity/kfire-server/internal/connectors/battlenet"
	"github.com/knightsofeternity/kfire-server/internal/connectors/steam"
	"github.com/knightsofeternity/kfire-server/internal/connectors/xbox"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/steamsync"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

// handlers carries the dependencies shared by every HTTP handler.
type handlers struct {
	cfg       *config.Config
	store     *store.Store
	hub       *ws.Hub
	steam     *steam.Connector
	steamSync *steamsync.Syncer
	battlenet *battlenet.Connector
	bnetSync  *bnetsync.Syncer
	xbox      *xbox.Connector
	cipher    *crypto.Cipher
}

// errorJSON writes the protocol's Error shape ({code, message}).
func errorJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"code": code, "message": message})
}

// rateLimiter limits a route to max requests/minute/IP.
func rateLimiter(max int) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderRetryAfter, "60")
			return errorJSON(c, fiber.StatusTooManyRequests, "rate_limited", "too many requests")
		},
	})
}

// Register mounts every route on the Fiber app.
func Register(app *fiber.App, cfg *config.Config, st *store.Store, hub *ws.Hub, steamConn *steam.Connector, syncer *steamsync.Syncer, cipher *crypto.Cipher) {
	bnConn := battlenet.New(cfg.BattlenetClientID, cfg.BattlenetClientSecret)
	if cfg.BattlenetOAuthBase != "" {
		bnConn.OAuthBase = cfg.BattlenetOAuthBase
	}
	bnConn.APIBase = cfg.BattlenetAPIBase
	bnetSync := bnetsync.New(st, bnConn, cipher, cfg.BattlenetRegion)
	xblConn := xbox.New(cfg.XblAppKey)
	if cfg.XblAPIBase != "" {
		xblConn.APIBase = cfg.XblAPIBase
	}
	h := &handlers{cfg: cfg, store: st, hub: hub, steam: steamConn, steamSync: syncer, battlenet: bnConn, bnetSync: bnetSync, xbox: xblConn, cipher: cipher}

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := app.Group("/api/v1")

	v1.Get("/config", h.publicConfig)

	// Sensitive endpoints: rate-limited per client IP (login brute force,
	// register spam). The group also carries token refresh, so keep some headroom.
	authGroup := v1.Group("/auth", rateLimiter(30))
	authGroup.Post("/register", h.register)
	authGroup.Post("/login", h.login)
	authGroup.Post("/refresh", h.refresh)
	authGroup.Post("/logout", h.requireAuth, h.logout)
	// Admin-generated password reset link (no email); the member sets a new
	// password from the link.
	authGroup.Get("/reset/:token", h.peekReset)
	authGroup.Post("/reset/:token", h.doReset)

	v1.Get("/users/me", h.requireAuth, h.me)
	v1.Patch("/users/me", h.requireAuth, h.updateMe)
	v1.Get("/users/:id", h.requireAuth, h.userProfile)
	v1.Get("/users/:id/games", h.requireAuth, h.userGames)
	v1.Get("/users/:id/games/:slug", h.requireAuth, h.userGameDetail)
	v1.Get("/users/:id/wow/:realm/:name/achievements", h.requireAuth, h.wowCharacterAchievements)
	v1.Get("/games", h.requireAuth, h.listGames)
	v1.Get("/games/played", h.requireAuth, h.listPlayedGames)
	v1.Get("/games/:slug", h.requireAuth, h.gameDetail)
	v1.Get("/presence", h.requireAuth, h.presence)
	v1.Get("/sessions", h.requireAuth, h.sessions)
	v1.Get("/achievements", h.requireAuth, h.userAchievements)

	// External account connectors. The OpenID callback is a public browser
	// redirect; it recovers the user from a signed state instead of a token.
	// Device pairing (browser-based client linking - OAuth device grant).
	// start/poll are public (the client isn't authenticated yet); approval is
	// done by the logged-in user in the web app.
	v1.Post("/devices/pair/start", rateLimiter(20), h.pairStart)
	v1.Post("/devices/pair/poll", h.pairPoll) // polled frequently; secured by a 256-bit device_code
	v1.Get("/devices/pair/:code", rateLimiter(20), h.requireAuth, h.pairInfo)
	v1.Post("/devices/pair/:code/approve", rateLimiter(20), h.requireAuth, h.pairApprove)

	v1.Get("/connect/steam", h.requireAuth, h.connectSteamStart)
	v1.Get("/connect/steam/callback", h.connectSteamCallback)
	v1.Post("/connect/steam/sync", h.requireAuth, h.syncSteam)
	v1.Delete("/connect/steam", h.requireAuth, h.disconnectSteam)

	v1.Get("/connect/battlenet", h.requireAuth, h.connectBattlenetStart)
	v1.Get("/connect/battlenet/callback", h.connectBattlenetCallback)
	v1.Delete("/connect/battlenet", h.requireAuth, h.disconnectBattlenet)

	v1.Get("/connect/xbox", h.requireAuth, h.connectXboxStart)
	v1.Get("/connect/xbox/callback", h.connectXboxCallback)
	v1.Delete("/connect/xbox", h.requireAuth, h.disconnectXbox)

	admin := v1.Group("/admin", h.requireAuth, h.requireAdmin)
	admin.Post("/games/sync", h.syncGames)
	admin.Get("/members", h.listMembers)
	admin.Patch("/members/:id", h.patchMember)
	admin.Post("/members/:id/reset", h.adminResetPassword)
	admin.Get("/invites", h.listInvites)
	admin.Post("/invites", h.createInvite)
	admin.Delete("/invites/:code", h.deleteInvite)
	admin.Get("/branding", h.getBranding)
	admin.Patch("/branding", h.setAccent)
	admin.Post("/branding/logo", h.uploadLogo)
	admin.Delete("/branding/logo", h.deleteLogo)

	admin.Post("/api-keys", h.createAPIKey)
	admin.Get("/api-keys", h.listAPIKeys)
	admin.Delete("/api-keys/:id", h.revokeAPIKey)

	// Public read-only API (API-key auth). Evaluated as a non-privileged viewer
	// so member privacy toggles apply. Rate-limited per key.
	pub := app.Group("/api/public/v1", h.requireAPIKey, apiKeyRateLimiter(120))
	pub.Get("/presence", h.publicPresence)

	// WebSocket upgrade for real-time presence. Authentication happens inside
	// the connection via the `hello` handshake (see kfire-protocol).
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(hub.Handler()))

	// Lazy image cache (public: referenced from <img> tags, no auth header).
	app.Get("/img/games/:id/:kind", h.gameImage)
	// Org logo (public: shown in the header and on the login screen).
	app.Get("/img/org/logo", h.orgLogo)
}

func notImplemented(c *fiber.Ctx) error {
	return errorJSON(c, fiber.StatusNotImplemented, "not_implemented",
		"this endpoint is not implemented yet")
}

// MountSPA serves the embedded admin SPA, falling back to index.html so the
// client-side router handles deep links. API and WebSocket paths are skipped.
// Mount this AFTER api.Register so real routes take precedence.
func MountSPA(app *fiber.App, dist fs.FS) {
	// Cache headers: content-hashed assets are immutable and safe to cache
	// forever; the entry HTML must never be cached (a new build changes which
	// hashed assets it references). A reverse proxy/CDN should honour these.
	app.Use(func(c *fiber.Ctx) error {
		p := c.Path()
		switch {
		case strings.HasPrefix(p, "/api/"), strings.HasPrefix(p, "/ws"), p == "/healthz":
			// not ours
		case strings.HasPrefix(p, "/_app/immutable/"):
			c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		default:
			c.Set(fiber.HeaderCacheControl, "no-cache")
		}
		return c.Next()
	})

	app.Use(filesystem.New(filesystem.Config{
		Root:         http.FS(dist),
		Index:        "index.html",
		NotFoundFile: "index.html", // SPA deep-link fallback
		Next: func(c *fiber.Ctx) bool {
			p := c.Path()
			return strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/ws") ||
				strings.HasPrefix(p, "/img/") || p == "/healthz"
		},
	}))
}
