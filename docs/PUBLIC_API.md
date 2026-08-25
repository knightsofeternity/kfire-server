# KFIRE Public API

HTTP API for external consumers (e.g. the knights-of-eternity site): read-only,
plus one invite-creation endpoint for invite-enabled keys.

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

Game-specific blocks (characters, profile blobs) are only present when the
corresponding game plugin is active on this instance. Before requesting per-game
detail, check `GET /api/v1/config` (no auth required) for the `game_plugins`
field -- it lists every currently active plugin and the slugs it covers:

    { "game_plugins": [
        { "id": "wow", "slugs": ["world-of-warcraft", "world-of-warcraft-classic"] },
        { "id": "d3",  "slugs": ["diablo-iii"] }
      ] }

If a plugin is absent from that list, its rich block will not appear in this
endpoint's response. See [docs/PLUGINS.md](./PLUGINS.md) for details.

### GET /games/{slug}
One game's aggregate: who's been playing it recently, and the all-time
leaderboard. Privacy is honored server-side: banned members are excluded from
both lists, hidden games return `404 {"code":"not_found"}`, and members who hid
their recent-session history (`sessions_visible=false`) are additionally
excluded from `recent_players` only — that setting does not affect aggregate
playtime, so they still appear in `all_time_players`.

    { "game": {"id":"…","name":"…","slug":"…","icon_url":"…","cover_url":"…"},
      "window_days": 7,
      "total_seconds": 123456,
      "player_count": 8,
      "recent_players": [
        { "user_id":"…", "username":"Ouranos", "avatar_url":"…", "total_seconds": 3600 }
      ],
      "all_time_players": [
        { "user_id":"…", "username":"Ouranos", "avatar_url":"…", "total_seconds": 54321, "session_count": 42 }
      ] }

`recent_players` covers a rolling 7-day window (completed sessions only),
sorted by playtime descending. `all_time_players` is the full leaderboard,
also sorted by playtime descending. `total_seconds` and `player_count` are
all-time totals (they describe `all_time_players`, not the 7-day window).
Unknown/hidden game → `404 {"code":"not_found"}`.

### POST /invites
Create a single-use KFIRE registration invite, so you can onboard a guild member
who does not have a KFIRE profile yet.

Requires an **invite-enabled** key (one created with "Peut créer des invitations"
checked). Ordinary read-only keys get `403 {"code":"forbidden"}`. This is the only
write endpoint on the public API.

Request body is optional and ignored; invites are always created with the
`member` role (admin invites cannot be issued over the public API).

    POST /invites
    -> 201 { "code": "Yx3…", "url": "https://kfire.io/?invite=Yx3…", "expires_at": "2026-06-29T10:00:00Z" }

The link stays valid for 14 days and behaves exactly like an admin-created invite:
the recipient opens `url`, registers, and joins as a member. Errors: `403
{"code":"forbidden"}` (read-only key), `401 {"code":"invalid_api_key"}` (bad key),
`429 {"code":"rate_limited"}` (over the per-key limit).
