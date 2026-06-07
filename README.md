# kfire-server

Backend and admin web UI for **KFIRE** (Knight FIRE) — an open-source, self-hosted
gaming presence tracker inspired by Xfire. One server instance = one organization
(clan, guild, team): see in real time who is playing what among your members, with
session history and statistics.

> **Status: auth implemented.** Registration, login (Argon2id + JWT),
> device-bound refresh token rotation, logout, `/users/me`, rate limiting and
> the WebSocket `hello` handshake all work against the contract defined in
> [kfire-protocol](https://github.com/knightsofeternity/kfire-protocol).
> Presence and session endpoints still return `501 Not Implemented`.

## Stack

- **Go + [Fiber](https://gofiber.io/)** — static binary, ~20 MB Docker image
- **PostgreSQL** — durable storage (users, sessions, linked accounts, games)
- **Redis** — presence state + WebSocket pub/sub
- **Caddy** — reverse proxy with automatic HTTPS (Let's Encrypt)
- Admin frontend (SvelteKit + Tailwind, dark gaming theme) — coming in `web/`

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
