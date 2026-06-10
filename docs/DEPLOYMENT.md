# Deployment

## Option A - standalone (bundled Caddy, auto-HTTPS)

For a host with a public IP and ports 80/443 free.

```bash
git clone https://github.com/knightsofeternity/kfire-server
cd kfire-server
cp .env.example .env       # fill in the values below
docker compose up -d --build
```

Caddy obtains a Let's Encrypt certificate for `KFIRE_DOMAIN` automatically - the domain must
resolve to the host and 80/443 must be reachable.

## Option B - behind an existing reverse proxy

For a host without a public IP, or that already runs a reverse proxy (Traefik, nginx, …).
Skips the bundled Caddy and publishes the server on an internal port.

```bash
cp .env.example .env       # + set KFIRE_BIND / KFIRE_HTTP_PORT if needed
docker compose -f docker-compose.proxied.yml up -d --build
```

**Which port does my proxy target?** The Go server listens on **8080 inside the container**.
`docker-compose.proxied.yml` publishes that on the host as `KFIRE_HTTP_PORT` (**default 8090**),
optionally bound to a private interface via `KFIRE_BIND`. So point your reverse proxy at
`http://<host>:8090` (or whatever `KFIRE_HTTP_PORT` you set) - not at 80/443, there is no Caddy
in this mode. A ready-to-edit Traefik file-provider config is in
[`deploy/traefik/kfire.yml`](../deploy/traefik). The Go server serves the API, WebSocket and
SPA on that one port; most proxies pass the WebSocket (`/ws`) through transparently.

**`KFIRE_DOMAIN` vs `KFIRE_PUBLIC_URL`:** in this compose file `KFIRE_PUBLIC_URL` is derived as
`https://${KFIRE_DOMAIN}` automatically, so you normally set **only `KFIRE_DOMAIN`**
(e.g. `kfire.example.org`). `KFIRE_PUBLIC_URL` must equal the public HTTPS URL the proxy serves
- it is the Steam OpenID realm and the base for image/pairing URLs - so only set it explicitly
if your public URL differs from `https://<KFIRE_DOMAIN>`.

## Environment

| Variable | Required | Notes |
|----------|----------|-------|
| `KFIRE_DATABASE_URL` | ✅ | PostgreSQL DSN |
| `KFIRE_JWT_SECRET` | ✅ | `openssl rand -hex 32` |
| `KFIRE_MASTER_KEY` | ✅ | `openssl rand -base64 32` (AES-256-GCM for OAuth tokens) |
| `KFIRE_DOMAIN` | compose | public domain (Caddy / proxy), e.g. `kfire.example.org` |
| `KFIRE_PUBLIC_URL` | auto | full `https://…` URL; derived from `KFIRE_DOMAIN` in the proxied compose - set only if different |
| `KFIRE_HTTP_PORT` | proxied | host port the server is published on (default `8090`); your proxy targets this |
| `KFIRE_BIND` | proxied | host interface to bind (default all); e.g. a private IP |
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
