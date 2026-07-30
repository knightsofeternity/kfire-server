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
	"github.com/knightsofeternity/kfire-server/internal/connectors/xbox"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/games"
	"github.com/knightsofeternity/kfire-server/internal/steamsync"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
	"github.com/knightsofeternity/kfire-server/internal/xboxsync"
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

	// Shared context for all background pollers.
	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()

	// Games catalog: seeded on first boot, then kept in step with Discord's
	// list. Runs in the background so startup stays fast when upstream is slow.
	go refreshCatalog(pollCtx, st)

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
		go syncer.Run(pollCtx, 6*time.Hour)
	}

	// AES-256-GCM cipher for OAuth tokens at rest.
	cipher, err := crypto.New(cfg.MasterKey)
	if err != nil {
		slog.Error("master key error", "err", err)
		os.Exit(1)
	}

	// Xbox connector + presence poller (dormant until KFIRE_XBL_APP_KEY is set).
	xblConn := xbox.New(cfg.XblAppKey)
	if cfg.XblAPIBase != "" {
		xblConn.APIBase = cfg.XblAPIBase
	}
	if xblConn.Enabled() {
		xs := xboxsync.New(st, xblConn, cipher, hub)
		go xs.Run(pollCtx, cfg.XboxPollInterval)
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

// Catalog refresh schedule. Upstream keeps adding games and fixing
// executables, so an instance seeded once on first boot silently drifts: games
// released since simply never detect for anyone.
const (
	catalogFirstCheck = 10 * time.Second   // fresh instance: seed almost immediately
	catalogCheckEvery = 6 * time.Hour      // cheap: one COUNT and one timestamp read
	catalogMaxAge     = 7 * 24 * time.Hour // upstream moves slowly; weekly is plenty
)

// refreshCatalog imports the Discord detectable-games catalog (~10k games) when
// it is missing or a week old, until the context is cancelled. The last import
// is stamped in the database, so the schedule survives restarts.
func refreshCatalog(ctx context.Context, st *store.Store) {
	var lastImport time.Time
	timer := time.NewTimer(catalogFirstCheck)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if reason := catalogImportReason(ctx, st, lastImport); reason != "" {
				importCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				n, err := importCatalog(importCtx, st)
				cancel()
				if err != nil {
					slog.Error("games catalog: import failed (retry via POST /api/v1/admin/games/sync)",
						"reason", reason, "err", err)
				} else {
					lastImport = time.Now()
					slog.Info("games catalog: imported", "reason", reason, "games", n)
				}
			}
			timer.Reset(catalogCheckEvery)
		}
	}
}

// catalogImportReason reports why the catalog should be imported now, or "" to
// leave it alone.
func catalogImportReason(ctx context.Context, st *store.Store, lastImport time.Time) string {
	if n, err := st.CountGames(ctx); err == nil && n == 0 {
		return "empty"
	}
	// Imported by this process recently. Checked before the stored timestamp so
	// an instance with no account yet (no org row to stamp, see
	// store.SetGamesSyncedAt) does not re-import on every tick.
	if !lastImport.IsZero() && time.Since(lastImport) < catalogMaxAge {
		return ""
	}
	syncedAt, ok, err := st.GamesSyncedAt(ctx)
	switch {
	case err != nil:
		slog.Error("games catalog: reading last sync", "err", err)
		return ""
	case !ok:
		return "never synced"
	case time.Since(syncedAt) >= catalogMaxAge:
		return "stale"
	default:
		return ""
	}
}

// importCatalog downloads the Discord list, upserts it and stamps the org.
func importCatalog(ctx context.Context, st *store.Store) (int, error) {
	seeds, err := games.FetchSeed(ctx)
	if err != nil {
		return 0, err
	}
	n, err := st.UpsertGames(ctx, seeds)
	if err != nil {
		return 0, err
	}
	if err := st.SetGamesSyncedAt(ctx); err != nil {
		// The catalog is updated; only the schedule bookkeeping failed.
		slog.Error("games catalog: stamping last sync", "err", err)
	}
	return n, nil
}
