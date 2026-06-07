// kfire-server — self-hosted gaming presence for organizations.
//
// One instance = one organization. See https://github.com/knightsofeternity/kfire-protocol
// for the client/server contract.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/knightsofeternity/kfire-server/internal/api"
	"github.com/knightsofeternity/kfire-server/internal/config"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database error", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		slog.Error("migration error", "err", err)
		os.Exit(1)
	}

	// TODO(mvp): connect Redis (presence + pub/sub) once the hub needs to
	// scale past a single process.

	app := fiber.New(fiber.Config{
		AppName:               "kfire-server",
		DisableStartupMessage: true,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	hub := ws.NewHub([]byte(cfg.JWTSecret))
	api.Register(app, cfg, st, hub)

	slog.Info("kfire-server listening", "addr", cfg.ListenAddr)
	if err := app.Listen(cfg.ListenAddr); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
