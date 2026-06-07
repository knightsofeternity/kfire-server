// Package api wires the REST control plane.
//
// Contract reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/openapi.yaml
package api

import (
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/knightsofeternity/kfire-server/internal/config"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

// handlers carries the dependencies shared by every HTTP handler.
type handlers struct {
	cfg   *config.Config
	store *store.Store
	hub   *ws.Hub
}

// errorJSON writes the protocol's Error shape ({code, message}).
func errorJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"code": code, "message": message})
}

// Register mounts every route on the Fiber app.
func Register(app *fiber.App, cfg *config.Config, st *store.Store, hub *ws.Hub) {
	h := &handlers{cfg: cfg, store: st, hub: hub}

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := app.Group("/api/v1")

	// Sensitive endpoints: 10 requests/min/IP (login brute force, register spam).
	authGroup := v1.Group("/auth", limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderRetryAfter, "60")
			return errorJSON(c, fiber.StatusTooManyRequests, "rate_limited", "too many requests")
		},
	}))
	authGroup.Post("/register", h.register)
	authGroup.Post("/login", h.login)
	authGroup.Post("/refresh", h.refresh)
	authGroup.Post("/logout", h.requireAuth, h.logout)

	v1.Get("/users/me", h.requireAuth, h.me)
	v1.Get("/games", h.requireAuth, h.listGames)
	v1.Get("/presence", h.requireAuth, h.presence)
	v1.Get("/sessions", h.requireAuth, h.sessions)

	admin := v1.Group("/admin", h.requireAuth, h.requireAdmin)
	admin.Post("/games/sync", h.syncGames)

	// WebSocket upgrade for real-time presence. Authentication happens inside
	// the connection via the `hello` handshake (see kfire-protocol).
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(hub.Handler()))
}

func notImplemented(c *fiber.Ctx) error {
	return errorJSON(c, fiber.StatusNotImplemented, "not_implemented",
		"this endpoint is not implemented yet")
}
