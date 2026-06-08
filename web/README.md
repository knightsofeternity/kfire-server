# kfire-server admin web UI

SvelteKit + Tailwind SPA (dark gaming theme) for KFIRE. Built to `web/build`,
embedded into the Go server binary via `//go:embed`, and served at `/`.

## Develop

```bash
pnpm install
pnpm dev      # http://localhost:5173 — proxies /api and /ws to the Go server (:8080)
```

Point at a different backend with `KFIRE_DEV_API=http://host:port pnpm dev`.

## Build

```bash
pnpm build    # writes web/build/, embedded by the next `go build` of the server
```

## Pages

- `/` — live dashboard (who's playing what), updated over WebSocket
- `/players` — member directory
- `/players/[id]` — player profile: per-game playtime, session history
- `/account` — privacy toggle (show/hide game activity), sign out
