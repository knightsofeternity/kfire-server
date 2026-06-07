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
	"github.com/knightsofeternity/kfire-server/internal/games"
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

	// First boot: seed the games catalog from Discord in the background so
	// startup stays fast even when the upstream is slow.
	if n, err := st.CountGames(ctx); err == nil && n == 0 {
		go seedGames(st)
	}

	app := fiber.New(fiber.Config{
		AppName:               "kfire-server",
		DisableStartupMessage: true,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	hub := ws.NewHub([]byte(cfg.JWTSecret), st)
	api.Register(app, cfg, st, hub)

	slog.Info("kfire-server listening", "addr", cfg.ListenAddr)
	if err := app.Listen(cfg.ListenAddr); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// seedGames imports the Discord detectable-games catalog (~10k games).
func seedGames(st *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	slog.Info("games seed: catalog empty, importing from Discord")
	seeds, err := games.FetchSeed(ctx)
	if err != nil {
		slog.Error("games seed: fetch failed (retry via POST /api/v1/admin/games/sync)", "err", err)
		return
	}
	n, err := st.UpsertGames(ctx, seeds)
	if err != nil {
		slog.Error("games seed: upsert failed", "err", err)
		return
	}
	slog.Info("games seed: done", "games", n)
}
