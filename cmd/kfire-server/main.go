// kfire-server - self-hosted gaming presence for organizations.
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
	"github.com/knightsofeternity/kfire-server/internal/connectors/steam"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/games"
	"github.com/knightsofeternity/kfire-server/internal/steamsync"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
	"github.com/knightsofeternity/kfire-server/web"
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
		// Behind Cloudflare + the Gate reverse proxy: use Cloudflare's
		// CF-Connecting-IP (the real client IP) so c.IP() and the per-IP rate
		// limiter key on the actual user, not the single proxy IP everyone shares.
		// Only trusted from the internal proxy network.
		ProxyHeader:             "CF-Connecting-IP",
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"127.0.0.1", "172.16.0.0/12", "10.0.0.0/8"},
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// Baseline security headers on every response (defense in depth).
	app.Use(func(c *fiber.Ctx) error {
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
		return c.Next()
	})

	hub := ws.NewHub([]byte(cfg.JWTSecret), st, cfg.PublicURL)

	// Steam connector + background library/achievement poller.
	steamConn := steam.New(cfg.SteamAPIKey)
	if cfg.SteamLoginBase != "" {
		steamConn.LoginBase = cfg.SteamLoginBase
	}
	if cfg.SteamAPIBase != "" {
		steamConn.APIBase = cfg.SteamAPIBase
	}
	syncer := steamsync.New(st, steamConn)
	if steamConn.Enabled() {
		pollCtx, cancelPoll := context.WithCancel(context.Background())
		defer cancelPoll()
		go syncer.Run(pollCtx, 6*time.Hour)
	}

	// AES-256-GCM cipher for OAuth tokens at rest.
	cipher, err := crypto.New(cfg.MasterKey)
	if err != nil {
		slog.Error("master key error", "err", err)
		os.Exit(1)
	}

	api.Register(app, cfg, st, hub, steamConn, syncer, cipher)

	// Serve the embedded admin SPA (when built). Mounted last so API and
	// WebSocket routes take precedence.
	if dist, ok := web.Dist(); ok {
		api.MountSPA(app, dist)
	} else {
		slog.Warn("admin SPA not built; serving API only (run `pnpm build` in web/)")
	}

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
