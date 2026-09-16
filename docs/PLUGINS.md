# Game plugins

A **game plugin** is a per-game rich integration that governs two things at once:
the **crawl** (API calls to a game-specific backend) and the **display blocks**
contributed to game pages and per-member game details. Plugins are distinct from
**connectors** (Steam, Battle.net, Xbox, Riot = the credential layer enabled by
environment variables). One connector can back several plugins: Battle.net backs
`wow`, `d3`, and `sc2`.

## The three states

| State | Meaning |
|-------|---------|
| **Registered** | The plugin is compiled into the binary. |
| **Available** | Its connector is configured (`Available()` returns `true`). |
| **Active** | Available **and** enabled in the `game_plugins` table. Only active plugins crawl and contribute display blocks. |

A plugin whose connector is absent is registered but never active, regardless of the
`enabled` flag in the database.

## Activation and admin control

Activation is per-instance and stored in the `game_plugins` table
(`migrations/0027_game_plugins.sql`). Each row has an `id` (e.g. `wow`) and an
`enabled` boolean.

**Default state:** the migration seeds `wow`, `d3`, and `sc2` as `enabled = true`
so live instances keep their rich blocks after an upgrade without any admin action.
A plugin that is newly registered at runtime but has no row yet will have a
default-enabled row inserted automatically at startup (`EnsurePluginDefaults`).

Admins control plugins at runtime through two endpoints (both require the `admin`
role):

### GET /api/v1/admin/plugins

Returns every registered plugin:

```json
{
  "plugins": [
    { "id": "wow",  "name": "World of Warcraft", "connector": "battlenet", "available": true, "enabled": true },
    { "id": "d3",   "name": "Diablo III",         "connector": "battlenet", "available": true, "enabled": true },
    { "id": "sc2",  "name": "StarCraft II",        "connector": "battlenet", "available": true, "enabled": false }
  ]
}
```

`available` reflects whether the underlying connector is configured on this
instance. `enabled` is the stored toggle. A plugin is only active when both are
`true`.

### PATCH /api/v1/admin/plugins/:id

Toggle a plugin on or off. Effect is immediate: the in-memory registry cache is
updated synchronously, no restart needed.

```json
{ "enabled": false }
```

Response: `{ "enabled": false }`. Unknown `id` returns `404 {"code":"not_found"}`.

## What disabling does

Disabling a plugin (`enabled: false`) suppresses exactly the rich, game-specific
content provided by that plugin. It does **not** affect generic presence.

| Surface | Effect when disabled |
|---------|---------------------|
| Game page (`/games/:slug`) | Plugin's aggregate block (e.g. `wow_characters`) is absent |
| Per-member game detail (`/users/:id/games/:slug`) | Plugin's per-member block is absent |
| Public API (`/api/public/v1/members/:id/games/:slug`) | Same -- plugin block absent |
| `game_plugins` in `/api/v1/config` | Plugin is dropped from the active list |
| Generic presence | **Unaffected** -- "X is playing World of Warcraft" still appears |

"Rich block" means: for WoW, the `wow_characters` list; for Diablo III and
StarCraft II, the `bnet_profile` blob. Playtime and session history (sourced
from the connector, not the plugin) continue to work regardless.

## `/api/v1/config` exposure

The public config endpoint includes a `game_plugins` field listing every
currently active plugin and the catalog slugs it owns:

```json
{
  "game_plugins": [
    { "id": "wow", "slugs": ["world-of-warcraft", "world-of-warcraft-classic"] },
    { "id": "d3",  "slugs": ["diablo-iii"] }
  ]
}
```

The SPA uses this list to decide which rich blocks to render. Downstream
consumers (for example, the Knights of Eternity site via the public API) should
also read it to know which game-specific data is currently available before
requesting per-member game detail.

## How to add a new plugin

1. **Implement `gameplugin.Plugin`** (`internal/gameplugin/plugin.go`). The
   interface requires `ID()`, `Name()`, `Connector()`, `Available()`, `Slugs()`,
   `Refresh()`, `GameDetail()`, and `UserGameDetail()`. Locate the implementation
   next to its connector's sync package (e.g. `internal/bnetsync/plugins.go`) to
   avoid import cycles -- the plugin package must not import `internal/api`.

2. **Register it in `api.Register`** (`internal/api/router.go`):

   ```go
   plugins.Register(bnetsync.NewWowPlugin(st, bnetSync, bnConn))
   ```

   Registration order determines the order in `List()` and `Active()`.

3. **Done for new games.** `plugins.Load()` calls `EnsurePluginDefaults` at
   startup, which inserts a default-enabled row for any plugin not yet in the
   `game_plugins` table. No migration is needed for a new plugin.

4. **Migrating an existing hardcoded game.** If the game previously contributed
   blocks through non-plugin code paths, ensure the new plugin returns blocks
   with the same JSON shape so existing clients and API consumers are not broken.

**League of Legends** (`lol`) requires the `riot` connector. It differs from the
other three plugins in three ways:

- This Riot product carries only an API key, with a personal key's rate
  limits, and no RSO application. So there is no OAuth flow: a member links by
  typing their Riot ID ("Name#TAG"), which the server resolves against Riot's
  Account-V1 endpoint. This does not prove the member owns the account, the
  same way public stats sites do not; it only checks that the Riot ID exists
  and that the resulting player identifier is not already linked to another
  member. Its connector's availability depends on a single environment
  variable, `KFIRE_RIOT_LOL_KEY`, the League of Legends API key.
- No token is kept for the member. Every read uses the server's own API key,
  so the link never expires. This is unlike Battle.net, where the member has
  to reconnect once their stored token goes stale.
- Disabling it also stops the background loop that polls for members currently
  in a League game, not just the two rich blocks (`lol_profile` and the live
  game indicator).

**Rate limit.** The application key is bound by Riot's real, application-wide
limit: twenty requests per second and one hundred per two minutes. Do not
confuse this with the per-method limits shown on the developer portal, those
are ceilings, not the actual grant. A League profile refresh costs eight
calls, is triggered only for the member viewing their own profile, and is
throttled to at most once per hour per member, so normal use stays well under
the limit. A 429 from Riot during a refresh is not fatal: nothing is written,
so the stored profile is left untouched and the next view simply retries
after the throttle window.

**Rocket League** (`rocket-league`) has no connector at all, unlike every
plugin above:

- `Connector()` returns `""` and `Available()` is always `true`. There is no
  credential layer to configure, so the admin switch in the `game_plugins`
  registry is the only way to turn this plugin off.
- It crawls nothing: `Refresh()` is inert. Results arrive over the WebSocket
  control plane as a `match_result` message that the desktop client sends
  when a match ends, not from any server-initiated pull. See "Routing a
  match result" in `docs/ARCHITECTURE.md` for how that message reaches
  `internal/rocketleague/recorder.go`.
- The desktop client obtains this from a TCP socket the game itself opens
  locally on the player's machine, enabled by a file named
  `DefaultStatsAPI.ini` in the game's install directory. This is entirely
  client-side; the server never talks to the game.
- `rocket_league_matches` (`migrations/0035_rocket_league_matches.sql`) has
  no free-text column except `result`, and `result` is a `CHECK`'d enum
  (`win`/`loss`/`draw`), not free text. This is deliberate: Rocket League's
  feed names every player in the match, team-mates and opponents alike, and
  the desktop client only ever reports facts about the member running it.
  The table is structurally incapable of holding a pseudonym, whatever a
  client sends -- there is no column to put one in.
- `playlist` is stored as Psyonix's raw integer id and never translated
  server-side, for the same reason `hero_card_id` is left untranslated for
  Hearthstone: the identifier is the fact, the label is presentation, and
  the label is localized.
- Turning the plugin off hides the guild record and the per-member match
  list (`rl_players`, `rl_matches`), leaving generic presence intact, like
  every other plugin.

**Disabling does not stop collection, for either Rocket League or
Hearthstone.** This is worth stating plainly because the opposite is the
natural assumption. `internal/matchrecord.Registry`, which routes an
incoming `match_result` to the recorder that can read it, has no knowledge of
the `game_plugins` admin switch above. It keeps writing matches to
`rocket_league_matches` (and to Hearthstone's table) while a plugin is
disabled and its blocks are hidden from every page. An admin flipping the
switch off can reasonably expect it to also stop the writes; it does not.
This has always been true of Hearthstone too, and was never written down
until now.

## Architecture pointers

Key files for the plugin system:

- `internal/gameplugin/plugin.go` -- `Plugin` interface and `Info` / `ActivePlugin` types
- `internal/gameplugin/registry.go` -- `Registry` (Register, Load, ForSlug, List, Active, ActivePlugins, SetEnabled)
- `internal/bnetsync/plugins.go` -- `WowPlugin` and `BnetProfilePlugin` (concrete implementations)
- `internal/api/router.go` -- registry construction and plugin registration in `Register`
- `internal/api/plugins_admin.go` -- `GET /admin/plugins` and `PATCH /admin/plugins/:id`
- `internal/api/admin.go` -- `game_plugins` field in `publicConfig`
- `migrations/0027_game_plugins.sql` -- table definition and seed data

The four handler surfaces gated on active plugins:

- `internal/api/games.go` (`gameDetail`) -- aggregate block on the game page
- `internal/api/player_game.go` (`userGameDetail`) -- per-member game detail
- `internal/api/public.go` (`publicMemberGameDetail`) -- public API game detail
- `internal/api/users.go` (`userProfile`) -- prefetch via `ActivePlugins()` when warming a member's data
