# Deployment

## Option A — standalone (bundled Caddy, auto-HTTPS)

For a host with a public IP and ports 80/443 free.

```bash
git clone https://github.com/knightsofeternity/kfire-server
cd kfire-server
cp .env.example .env       # fill in the values below
docker compose up -d --build
```

Caddy obtains a Let's Encrypt certificate for `KFIRE_DOMAIN` automatically — the domain must
resolve to the host and 80/443 must be reachable.

## Option B — behind an existing reverse proxy

For a host without a public IP, or that already runs a reverse proxy (Traefik, nginx, …).
Skips the bundled Caddy and publishes the server on an internal port.

```bash
cp .env.example .env       # + set KFIRE_BIND / KFIRE_HTTP_PORT if needed
docker compose -f docker-compose.proxied.yml up -d --build
```

Then route your proxy to the published port. A ready-to-edit Traefik file-provider config is
in [`deploy/traefik/kfire.yml`](../deploy/traefik). The Go server serves the API, WebSocket
and SPA on one port; most proxies pass WebSocket through transparently.

`KFIRE_PUBLIC_URL` must equal the public HTTPS URL the proxy serves (it is the Steam OpenID
realm and the base for image/pairing URLs).

## Environment

| Variable | Required | Notes |
|----------|----------|-------|
| `KFIRE_DATABASE_URL` | ✅ | PostgreSQL DSN |
| `KFIRE_JWT_SECRET` | ✅ | `openssl rand -hex 32` |
| `KFIRE_MASTER_KEY` | ✅ | `openssl rand -base64 32` (AES-256-GCM for OAuth tokens) |
| `KFIRE_DOMAIN` | compose | public domain (Caddy / proxy) |
| `KFIRE_PUBLIC_URL` | recommended | full `https://…` public URL |
| `KFIRE_ORG_NAME` | | organization display name |
| `KFIRE_OPEN_REGISTRATION` | | `false` = invite-only (recommended) |
| `KFIRE_STEAM_API_KEY` | optional | enables the Steam connector |

## First run

- Migrations apply automatically; the games catalog seeds from Discord on first boot.
- The **first registered account becomes the admin** (works even when registration is
  invite-only). Create it on the website, then invite members from **Admin → Invite**.

## Updating

```bash
git pull
docker compose -f docker-compose.proxied.yml up -d --build --force-recreate server
```

If you front the app with a CDN that caches HTML, purge it after a deploy (the server sends
`Cache-Control: no-cache` on the entry HTML and `immutable` on hashed assets).
