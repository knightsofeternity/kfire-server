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
  (~10k games, executables for matching, icon/cover art, Steam app ids).
- **Image cache** (`/img/games/:id/:kind`) - lazily fetches & stores game icons/covers in
  Postgres on first request, so storage scales with games actually shown.
- **Steam** (`internal/connectors/steam`, `internal/steamsync`) - OpenID account linking +
  background import of library playtime and achievements.
- **Admin SPA** (`web/`) - SvelteKit + Tailwind, built and embedded via `//go:embed`.

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
