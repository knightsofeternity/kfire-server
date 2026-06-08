# kfire-server

Backend and admin web UI for **KFIRE** (Knight FIRE) — an open-source, self-hosted
gaming presence tracker inspired by Xfire. One server instance = one organization
(clan, guild, team): see in real time who is playing what among your members, with
session history and statistics.

> **Status: MVP backend + admin UI.** Auth (Argon2id + JWT, device-bound
> refresh rotation), real-time presence over WebSocket, persisted sessions
> with history, a ~10k games catalog seeded from Discord, player profiles with
> per-game playtime, an activity privacy toggle, the embedded admin web UI,
> and **Steam account linking** (OpenID + Web API). Everything follows the
> contract in [kfire-protocol](https://github.com/knightsofeternity/kfire-protocol).

## Stack

- **Go + [Fiber](https://gofiber.io/)** — static binary, ~20 MB Docker image
- **PostgreSQL** — durable storage (users, sessions, linked accounts, games)
- **Redis** — presence state + WebSocket pub/sub
- **Caddy** — reverse proxy with automatic HTTPS (Let's Encrypt)
- **Admin web UI** — SvelteKit + Tailwind SPA (dark gaming theme) in `web/`,
  embedded into the Go binary and served at `/`: live dashboard, player
  profiles with per-game playtime, account settings (activity privacy toggle)

## Self-hosting (Docker)

```bash
git clone https://github.com/knightsofeternity/kfire-server
cd kfire-server
cp .env.example .env
# Fill in .env: domain, postgres password, JWT secret, master key
docker compose up -d
```

Caddy obtains a Let's Encrypt certificate for `KFIRE_DOMAIN` automatically —
ports 80/443 must be reachable and the domain must resolve to your host.

## Local development

Requires Go ≥ 1.23 and Docker (for Postgres/Redis).

```bash
docker compose up -d postgres redis

export KFIRE_DATABASE_URL="postgres://kfire:dev@localhost:5432/kfire?sslmode=disable"
export KFIRE_JWT_SECRET=$(openssl rand -hex 32)
export KFIRE_MASTER_KEY=$(openssl rand -base64 32)

go run ./cmd/kfire-server
curl localhost:8080/healthz
```

### Admin web UI

The SvelteKit SPA lives in [`web/`](./web) and is embedded into the binary via
`//go:embed`. For frontend development run it with hot reload against the Go
server (Vite proxies `/api` and `/ws`):

```bash
cd web
pnpm install
pnpm dev            # http://localhost:5173, proxying to :8080
```

For a production build (also what Docker does), `pnpm build` writes `web/build`,
which the next `go build` embeds and serves at `/`. The binary runs API-only if
the SPA was never built.

Migrations live in [`migrations/`](./migrations) (plain SQL, embedded in the
binary and applied automatically at startup).

The first registered account becomes the instance **admin**. Set
`KFIRE_OPEN_REGISTRATION=false` to close registration after that
(invite system is TODO).

## Project layout

```
cmd/kfire-server/   entrypoint
internal/config/    environment configuration
internal/api/       REST routes (contract: kfire-protocol/openapi.yaml)
internal/ws/        WebSocket presence hub (contract: kfire-protocol/websocket-events.md)
migrations/         SQL schema migrations
deploy/             Caddyfile
```

## Connectors

External platform accounts link from the **Account** page. Implemented:

- **Steam** — "Sign in through Steam" (OpenID 2.0) yields the SteamID; the
  server's Steam Web API key (`KFIRE_STEAM_API_KEY`) resolves the persona and
  avatar, and imports the library (lifetime playtime, merged into each profile's
  per-game stats) and unlocked **achievements** for the most-played games. A
  background poller refreshes every linked account every 6 h; members can also
  trigger a sync from their Account page. No per-user secret is stored. Without
  the key the connector returns `501`.

Planned next: Battle.net, Riot, Epic (OAuth2), Xbox (OpenXBL), PlayStation
(psn-api, best-effort) — those use OAuth2, so their tokens will be encrypted at
rest with AES-256-GCM (`KFIRE_MASTER_KEY`).

## Security model

- Passwords hashed with **Argon2id** (never bcrypt)
- **JWT 15 min** access tokens + single-use, device-bound refresh tokens
- OAuth tokens (Steam, Battle.net, …) encrypted at rest with **AES-256-GCM**,
  master key in env only
- HTTPS enforced by Caddy; HSTS enabled
- Rate limiting on sensitive endpoints (login, OAuth)
- GDPR from the MVP: account export + deletion endpoints

## Related repositories

- [kfire-protocol](https://github.com/knightsofeternity/kfire-protocol) — API & WebSocket contract (Apache-2.0)
- [kfire-client](https://github.com/knightsofeternity/kfire-client) — desktop tray client (MIT)

## License

[AGPL-3.0](./LICENSE) — if you host a modified version, you must publish your changes.
