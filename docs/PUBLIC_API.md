# KFIRE Public API

Read-only HTTP API for external consumers (e.g. the knights-of-eternity site).

## Authentication

Every request needs an API key (minted by an admin under **Admin → Clés API**)
sent as a bearer token. Keys are secrets: use them **server-to-server only**
(your backend fetches and caches); never expose a key in browser JavaScript.

    Authorization: Bearer kfire_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

Base URL: `https://<your-kfire-host>/api/public/v1`

Rate limit: 120 requests/minute/key → `429 {"code":"rate_limited"}` with
`Retry-After: 60`. Invalid/revoked/missing key → `401 {"code":"invalid_api_key"}`.

## Privacy

The API behaves like a regular (non-admin) member viewer. A member who disabled
**activity visibility** does not expose their current game; one who disabled
**session visibility** does not expose playtime, stats, or library. Name and
avatar are always public. Banned members are not returned.

## Endpoints

### GET /presence
Who is online and what they're playing.

    { "entries": [
      { "user_id": "…", "username": "Ouranos", "avatar_url": "…",
        "status": "in_game", "game": {"id":"…","name":"…","slug":"…","icon_url":"…"}, "since": "2026-06-14T…Z" }
    ] }

`status` is `in_game` | `online` | `offline`.

### GET /members
Roster with linked-account summaries (for mapping to your own users).

    { "members": [
      { "id":"…", "username":"Ouranos", "avatar_url":"…",
        "connections": [ {"provider":"steam","display_name":"…","provider_user_id":"7656…","profile_url":"…"} ] }
    ] }

### GET /members/{id}
Profile: presence, connections, and (if the member shares sessions) per-game
playtime stats and total. Unknown/banned member → `404 {"code":"not_found"}`.

### GET /members/{id}/games
The member's library (`{ "games": [ {"game":{…},"source":"steam|battlenet|played"} ] }`),
empty if the member hid their sessions.

### GET /members/{id}/games/{slug}
Per-game detail: playtime (if shared), WoW characters, and the Diablo III /
StarCraft II profile blob (`bnet_profile`) when present. Serves cached data only.
