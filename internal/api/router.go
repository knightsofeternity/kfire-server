// Package api wires the REST control plane.
//
// Contract reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/openapi.yaml
package api

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/config"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

// Register mounts every route on the Fiber app.
func Register(app *fiber.App, cfg *config.Config, hub *ws.Hub) {
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := app.Group("/api/v1")

	auth := v1.Group("/auth")
	// TODO(mvp): rate-limit the auth group (login brute force protection).
	auth.Post("/register", notImplemented)
	auth.Post("/login", notImplemented) // Argon2id verify + JWT 15 min + device-bound refresh token
	auth.Post("/refresh", notImplemented)
	auth.Post("/logout", notImplemented)

	// TODO(mvp): JWT middleware guarding everything below.
	v1.Get("/users/me", notImplemented)
	v1.Get("/presence", notImplemented)
	v1.Get("/sessions", notImplemented)

	// WebSocket upgrade for real-time presence.
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(hub.Handler()))
}

func notImplemented(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"code":    "not_implemented",
		"message": "this endpoint is not implemented yet",
	})
}
