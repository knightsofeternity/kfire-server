// kfire-server — self-hosted gaming presence for organizations.
//
// One instance = one organization. See https://github.com/knightsofeternity/kfire-protocol
// for the client/server contract.
package main

import (
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/knightsofeternity/kfire-server/internal/api"
	"github.com/knightsofeternity/kfire-server/internal/config"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}

	// TODO(mvp): open PostgreSQL (pgx pool) and Redis connections here,
	// run pending migrations, then inject the stores into api/ws.

	app := fiber.New(fiber.Config{
		AppName:               "kfire-server",
		DisableStartupMessage: true,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	hub := ws.NewHub()
	api.Register(app, cfg, hub)

	slog.Info("kfire-server listening", "addr", cfg.ListenAddr)
	if err := app.Listen(cfg.ListenAddr); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
