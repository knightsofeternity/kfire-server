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

The next planned plugin is **League of Legends** (`lol`), which requires the
Riot connector to be available.

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
