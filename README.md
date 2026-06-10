<p align="center">
  <img src=".github/logo.png" alt="KFIRE" width="300" />
</p>

<h1 align="center">kfire-server</h1>

<p align="center">
  Self-hosted gaming presence for your organization - see who's playing what, in real time.
</p>

<p align="center">
  <a href="./LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/badge/license-AGPL--3.0-fb923c"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-compose-2496ED?logo=docker&logoColor=white">
  <a href="https://github.com/knightsofeternity/kfire-protocol"><img alt="Protocol" src="https://img.shields.io/badge/API-OpenAPI%203.1-6BA539?logo=openapiinitiative&logoColor=white"></a>
</p>

---

**KFIRE** (Knight FIRE) is an open-source, self-hosted gaming presence tracker inspired by
[Xfire](https://en.wikipedia.org/wiki/Xfire) (2004–2015). One server instance = one
organization (clan, guild, team): your members run a lightweight desktop client, and you
see - in real time - who is playing what, with history, stats and internal leaderboards.

This repository is the **backend + admin web UI**. It ships as a single static Go binary
(the SvelteKit admin SPA is embedded) and self-hosts with Docker.

## Features

- 🎮 **Live presence** - real-time "who's playing what", over WebSocket
- 📊 **Profiles & stats** - per-game playtime, session history, achievements
- 🏆 **Per-game leaderboards** - who plays the most in your org
- 🗂️ **~10k games catalog** - auto-seeded from Discord's detectable-apps list, with cached artwork
- 🔗 **Steam & Battle.net connectors** - link your accounts, import Steam library playtime & achievements
- 🌐 **Multi-guild client** - one desktop app reports to several KFIRE servers at once, one per guild
- 🔒 **Privacy & status** - go online / invisible / offline from the client (global or per guild), plus per-member web toggles to hide your current game or your recent-sessions history
- 👑 **Member management** - invite links, roles, ban; invite-only registration
- 🖥️ **Frictionless client linking** - approve devices from the browser, no password in the app
- 🛡️ **Secure by default** - Argon2id, short JWTs + device-bound refresh tokens, HTTPS

## Stack

Go + [Fiber](https://gofiber.io) · PostgreSQL · Redis · [SvelteKit](https://svelte.dev) +
Tailwind (embedded) · Caddy (auto-HTTPS).

## Self-hosting

```bash
git clone https://github.com/knightsofeternity/kfire-server
cd kfire-server
cp .env.example .env          # fill in the secrets (openssl commands are in the file)
docker compose up -d          # server + postgres + redis + caddy (auto-HTTPS)
```

The first account you create becomes the **admin**. From there, invite your members and
point them to the **download page** for the desktop client.

> **Behind an existing reverse proxy** (no public IP / shared host)? Use
> `docker-compose.proxied.yml` - see the [deployment guide](./docs/DEPLOYMENT.md).

## Documentation

- 📐 [Architecture](./docs/ARCHITECTURE.md)
- 🚀 [Deployment](./docs/DEPLOYMENT.md)
- 🧭 [Player onboarding](./docs/ONBOARDING.md)
- 📜 [API & protocol](https://github.com/knightsofeternity/kfire-protocol)

## Related repositories

| Repo | What | License |
|------|------|---------|
| [kfire-server](https://github.com/knightsofeternity/kfire-server) | Backend + admin web UI | AGPL-3.0 |
| [kfire-client](https://github.com/knightsofeternity/kfire-client) | Desktop tray client | MIT |
| [kfire-protocol](https://github.com/knightsofeternity/kfire-protocol) | API & WebSocket contract | Apache-2.0 |

## Development

Requires Go ≥ 1.25, Node ≥ 22 + pnpm, and Docker.

```bash
docker compose up -d postgres redis
export KFIRE_DATABASE_URL="postgres://kfire:dev@localhost:5432/kfire?sslmode=disable"
export KFIRE_JWT_SECRET=$(openssl rand -hex 32)
export KFIRE_MASTER_KEY=$(openssl rand -base64 32)
go run ./cmd/kfire-server          # API only
cd web && pnpm install && pnpm dev # admin UI with hot reload (proxies to :8080)
```

## License

[AGPL-3.0](./LICENSE) - if you host a modified version, you must publish your changes.
