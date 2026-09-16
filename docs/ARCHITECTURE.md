# Architecture

KFIRE is mono-tenant: **one server instance = one organization**. It self-hosts with Docker
and ships as a single static Go binary with the admin SPA embedded.

```
                         ┌──────────────────────────────────────────┐
   Desktop clients       │                kfire-server               │
   (Tauri, per member)   │                                           │
        │  WebSocket /ws  │   Fiber HTTP ── REST API (/api/v1)        │
        │  REST  /api     │      │      ├─ WebSocket hub (presence)   │
        ├────────────────▶│      │      ├─ image cache proxy (/img)   │
        │                 │      │      └─ embedded admin SPA (/)      │
   Web admin (browser) ──▶│      │                                    │
        │                 │   stores ── PostgreSQL (durable)          │
        │                 │           └─ Redis (presence/pubsub) *    │
        │                 │   connectors ── Steam (OpenID + Web API)  │
        │                 │   poller ─────── Steam library/achievements│
                          └──────────────────────────────────────────┘
   * Redis is wired in compose; the in-process hub is the current source of truth.
```

## Components

- **REST API** (`internal/api`) - auth, device pairing, users/profiles, games, presence
  snapshot, sessions, connectors, admin (members/invites). Contract:
  [kfire-protocol/openapi.yaml](https://github.com/knightsofeternity/kfire-protocol).
- **WebSocket hub** (`internal/ws`) - real-time presence. Clients authenticate with a
  `hello` handshake (JWT), send `game_started`/`game_stopped`/`heartbeat`; the hub persists
  sessions and broadcasts `presence_update`.
- **Store** (`internal/store`) - PostgreSQL via pgx; embedded SQL migrations applied at boot.
- **Games catalog** (`internal/games`) - seeded from Discord's public detectable-apps list
  (~10k games, executables for matching, icon/cover art, Steam app ids). Imported on first
  boot, then refreshed in the background when older than a week (`orgs.games_synced_at`),
  or on demand from the admin SPA. See "Game detection" below.
- **Image cache** (`/img/games/:id/:kind`) - lazily fetches & stores game icons/covers in
  Postgres on first request, so storage scales with games actually shown.
- **Steam** (`internal/connectors/steam`, `internal/steamsync`) - OpenID account linking +
  background import of library playtime and achievements.
- **Admin SPA** (`web/`) - SvelteKit + Tailwind, built and embedded via `//go:embed`.

## Game detection

`games.executable_names` carries two kinds of entries, and the desktop client matches a
running process against both:

- a **basename**, e.g. `wow.exe`, compared to the process name. Robust across stores and
  install locations, so it is always preferred.
- a **qualified path pattern**, e.g. `counter-strike source/hl2.exe`, matched as a suffix
  of the process's full path (segment-aligned, case-insensitive, forward slashes). Used
  only when the basename cannot identify the game on its own.

Two denylists decide which form applies (`internal/games/discord.go`):

- `neverDetect` - installers, updaters, config tools, crash handlers, Windows system
  binaries, anti-cheat bootstrappers. Dropped in both forms: installing or configuring a
  game is not playing it.
- `ambiguousExecutables` - real game binaries with a name too common to identify anything
  (`game.exe`, `launcher.exe`, script runtimes). Dropped as basenames, kept as qualified
  patterns, which is what makes ~470 otherwise invisible games detectable.

A basename shared by more than `maxGamesPerExecutable` games (e.g. `hl2.exe`, ~34 Source
games) is dropped too, and rescued the same way; the frequency rule applies to patterns as
well, so a pattern stays useful only while it discriminates.

Clients before v0.4.0 ignore pattern entries: they index them like any other name and
never match, so publishing patterns is safe for a mixed fleet.

## Match results and the live match

Two WebSocket messages carry per-game data that the hub itself does not
understand: `match_result`, a finished match reported by the desktop client,
and `live_match`, the score while a match is still in progress.

**Routing a match result.** `match_result` carries a `game_slug`; the hub
(`internal/ws/hub.go`, `handleMatchResult`) reads only that slug and hands the
whole payload, undecoded, to `internal/matchrecord.Registry`, which resolves
it to the `Recorder` that claims that slug (`hearthstone.NewRecorder`,
`rocketleague.NewRecorder`, built in `cmd/kfire-server/main.go`). The hub
itself knows no game: adding a third game means writing a `Recorder`
(`Slug()` + `Record()`) and registering it in that list, without touching
`hub.go`.

**Two registries, easy to confuse.** `internal/gameplugin` and
`internal/matchrecord` are both registries a new game gets added to, and they
answer different questions:

- `internal/gameplugin.Registry` governs what the pages display -- it carries
  the admin on/off switch (`game_plugins` table) and the crawl. See
  [PLUGINS.md](PLUGINS.md). A plugin is registered in `internal/api/router.go`.
- `internal/matchrecord.Registry` only routes an incoming match to the code
  that can read it. It has no admin switch and does not know whether the
  matching plugin is enabled. A recorder is registered separately, in
  `cmd/kfire-server/main.go`.

A game that reports match results is registered in **both**, in two
different files, for two different reasons. Disabling the plugin hides the
display; it does not stop the recorder from writing, because the recorder
never consults the plugin registry. See [PLUGINS.md](PLUGINS.md) for the
consequence this has for Hearthstone and Rocket League.

**The live match** (`live_match`) is the ephemeral counterpart: the current
score, broadcast to the hub's normal presence channel while a match is still
running, and never written to the database (`internal/ws/live.go`,
`handleLiveMatch`). It is kept in memory next to presence, expires if no
update refreshes it within `liveTTL`, and is swept on a timer
(`Hub.SweepLive`, started from `cmd/kfire-server/main.go`) so a game that
crashes without closing the socket doesn't leave a frozen score on screen
forever. It is also cut immediately, not just on expiry, the moment a member
chooses to be invisible (`Hub.SetVisibility`): a member who asks not to be
seen must stop being seen right away, not at the next reconnect.

## Key data

`orgs`, `users`, `refresh_tokens` (device-bound), `device_pairings`, `invites`, `games`,
`linked_accounts`, `game_sessions`, `external_playtime`, `achievements`, `image_cache`.

## Security model

- Passwords: **Argon2id**. Login is timing-oracle-safe; weak/common passwords rejected (NIST).
- Tokens: **15-minute JWT** access + single-use, **device-bound refresh tokens** (rotated).
- Client linking: **OAuth device grant** - approved from the browser, never a password in the app.
- OAuth secrets at rest: **AES-256-GCM** (master key in env). *(Steam needs no per-user secret.)*
- HTTPS enforced (Caddy, or your reverse proxy). Rate limiting on `/auth`.
- Privacy: per-member toggle hides the current game from other members.
