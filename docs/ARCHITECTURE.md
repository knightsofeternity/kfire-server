# Architecture

KFIRE is mono-tenant: **one server instance = one organization**. It self-hosts with Docker
and ships as a single static Go binary with the admin SPA embedded.

```
                         ┌──────────────────────────────────────────┐
   Desktop clients       │                kfire-server               │
   (Tauri, per member)   │                                           │
        │  WebSocket /ws  │   Fiber HTTP ── REST API (/api/v1)        │
        │  REST  /api     │      │      ├─ WebSocket hub (presence)   │
        ├────────────────▶│      │      ├─ image cache proxy (/img)   │
        │                 │      │      └─ embedded admin SPA (/)      │
   Web admin (browser) ──▶│      │                                    │
        │                 │   stores ── PostgreSQL (durable)          │
        │                 │           └─ Redis (presence/pubsub) *    │
        │                 │   connectors ── Steam (OpenID + Web API)  │
        │                 │   poller ─────── Steam library/achievements│
                          └──────────────────────────────────────────┘
   * Redis is wired in compose; the in-process hub is the current source of truth.
```

## Components

- **REST API** (`internal/api`) - auth, device pairing, users/profiles, games, presence
  snapshot, sessions, connectors, admin (members/invites). Contract:
  [kfire-protocol/openapi.yaml](https://github.com/knightsofeternity/kfire-protocol).
- **WebSocket hub** (`internal/ws`) - real-time presence. Clients authenticate with a
  `hello` handshake (JWT), send `game_started`/`game_stopped`/`heartbeat`; the hub persists
  sessions and broadcasts `presence_update`.
- **Store** (`internal/store`) - PostgreSQL via pgx; embedded SQL migrations applied at boot.
- **Games catalog** (`internal/games`) - seeded from Discord's public detectable-apps list
  (~10k games, executables for matching, icon/cover art, Steam app ids). Imported on first
  boot, then refreshed in the background when older than a week (`orgs.games_synced_at`),
  or on demand from the admin SPA. See "Game detection" below.
- **Image cache** (`/img/games/:id/:kind`) - lazily fetches & stores game icons/covers in
  Postgres on first request, so storage scales with games actually shown.
- **Steam** (`internal/connectors/steam`, `internal/steamsync`) - OpenID account linking +
  background import of library playtime and achievements.
- **Admin SPA** (`web/`) - SvelteKit + Tailwind, built and embedded via `//go:embed`.

## Game detection

`games.executable_names` carries two kinds of entries, and the desktop client matches a
running process against both:

- a **basename**, e.g. `wow.exe`, compared to the process name. Robust across stores and
  install locations, so it is always preferred.
- a **qualified path pattern**, e.g. `counter-strike source/hl2.exe`, matched as a suffix
  of the process's full path (segment-aligned, case-insensitive, forward slashes). Used
  only when the basename cannot identify the game on its own.

Two denylists decide which form applies (`internal/games/discord.go`):

- `neverDetect` - installers, updaters, config tools, crash handlers, Windows system
  binaries, anti-cheat bootstrappers. Dropped in both forms: installing or configuring a
  game is not playing it.
- `ambiguousExecutables` - real game binaries with a name too common to identify anything
  (`game.exe`, `launcher.exe`, script runtimes). Dropped as basenames, kept as qualified
  patterns, which is what makes ~470 otherwise invisible games detectable.

A basename shared by more than `maxGamesPerExecutable` games (e.g. `hl2.exe`, ~34 Source
games) is dropped too, and rescued the same way; the frequency rule applies to patterns as
well, so a pattern stays useful only while it discriminates.

Clients before v0.4.0 ignore pattern entries: they index them like any other name and
never match, so publishing patterns is safe for a mixed fleet.

## Match results and the live match

Two WebSocket messages carry per-game data that the hub itself does not
understand: `match_result`, a finished match reported by the desktop client,
and `live_match`, the score while a match is still in progress.

**Routing a match result.** `match_result` carries a `game_slug`; the hub
(`internal/ws/hub.go`, `handleMatchResult`) reads only that slug and hands the
whole payload, undecoded, to `internal/matchrecord.Registry`, which resolves
it to the `Recorder` that claims that slug (`hearthstone.NewRecorder`,
`rocketleague.NewRecorder`, built in `cmd/kfire-server/main.go`). The hub
itself knows no game: adding a third game means writing a `Recorder`
(`Slug()` + `Record()`) and registering it in that list, without touching
`hub.go`.

**Three registries, easy to confuse.** `internal/gameplugin`,
`internal/matchrecord` and `internal/livestate` are all registries a new game
gets added to, and they answer different questions:

- `internal/gameplugin.Registry` governs what the pages display -- it carries
  the admin on/off switch (`game_plugins` table) and the crawl. See
  [PLUGINS.md](PLUGINS.md). A plugin is registered in `internal/api/router.go`.
- `internal/matchrecord.Registry` only routes an incoming match to the code
  that can read it. It has no admin switch and does not know whether the
  matching plugin is enabled. A recorder is registered separately, in
  `cmd/kfire-server/main.go`.
- `internal/livestate.Registry` does the same for a match still in progress,
  and is registered in the same place. A game can be in one and not the
  other: a live state and a recorded result are independent.

A game that reports match results is registered in **both**, in two
different files, for two different reasons. Disabling the plugin hides the
display; it does not stop the recorder from writing, because the recorder
never consults the plugin registry. See [PLUGINS.md](PLUGINS.md) for the
consequence this has for Hearthstone and Rocket League.

**The live match** (`live_match`) is the ephemeral counterpart: the state of a
match still in progress, broadcast to the hub's normal presence channel and
never written to the database. It is routed exactly like a match result, and
for the same reason: `internal/livestate.Registry` resolves the payload's
`game_slug` to the `Reporter` that claims it (`rocketleague.NewLiveReporter`),
and the hub knows no game's fields. A `Reporter` validates the game's own
payload and returns what will be broadcast, so a game never broadcasts more
than it has validated. Adding a game to the live page means writing a
`Reporter` and registering it in `cmd/kfire-server/main.go`, without touching
`hub.go`.

Three things stay common, and are checked before any reporter sees anything:
the slug's shape, the generic end-of-match signal (`"ended": true`), and a
**4 KiB cap on the payload**, enforced both on what the client sends and on
what the reporter returns. The cap exists because this is the only message in
KFIRE that takes data from one member's machine and relays it to everyone
else's screen: a hostile or broken client must not be able to flood the
guild's browsers.

**A live state does not have to come from a client.** Rocket League is PUSHED by the
member's own machine over this socket; League of Legends is PULLED by the server from
Riot's Spectator API, and reaches the hub through `Hub.PublishLive` instead. The visibility
rule is identical either way: a member who asked not to be seen is not seen, whoever the
state came from. A pulled state is not passed through `livestate.Registry`, because a
registry exists to shape what an untrusted client sent, and this one is built by our own
code from a typed answer.

Two consequences worth knowing:

- **The source declares its own lifetime** (`State.TTL`). The default is sized for a client
  sampling twice a second; a poller running once a minute would otherwise be swept away
  between two samples and its card would blink.
- **Visibility is read from the database when the hub has never seen the member on a
  connection.** `liveVisible` is filled at authentication, which was enough while every
  state arrived over a socket. A member playing League with no KFIRE client running has no
  entry, and a plain map lookup would read the zero value and refuse them forever, making
  the whole pulled path silently mute in exactly the case it exists to serve.

The state is kept in memory next to presence, expires if no
update refreshes it within `liveTTL`, and is swept on a timer
(`Hub.SweepLive`, started from `cmd/kfire-server/main.go`) so a game that
crashes without closing the socket doesn't leave a frozen score on screen
forever. It is also cut immediately, not just on expiry, the moment a member
chooses to be invisible (`Hub.SetVisibility`): a member who asks not to be
seen must stop being seen right away, not at the next reconnect.

## The Riot quota

KFIRE holds a personal Riot API key, whose binding limit is about 100 calls per two
minutes. **One limiter, inside `riot.Connector.get`**, paces every call: the hourly refresh,
the live poller and the history backfill all queue behind the same cursor, because the quota
belongs to the key and not to a caller.

It is a spacing limiter rather than a token bucket on purpose: a bucket lets a burst
through, and a burst is what gets a key throttled. A 429 answer pushes EVERY pending caller
back, not just the one that was refused, and is then retried: a 429 means "later", not "no".

Being throttled is expensive out of proportion to being slow, because it hits every League
surface at once, including the live poller that other members are watching.

**The backfill is the lowest priority consumer.** It walks a member's whole match history,
one page of 100 at a time, every two minutes, and it can take hours. Two properties make
that acceptable: its cursor is stored after every page, so a restart resumes instead of
starting over; and a transient failure abandons the page WITHOUT moving the cursor. That
last point is not a detail -- skipping a match on a rate limit would move the cursor past
history that nothing would ever walk again, carving a permanent hole nobody could audit.

## The PUBG quota, and why it is not the Riot one

PUBG allows **10 requests per minute**, tighter than Riot. But **`/matches` and telemetry do
not count against it**, which was confirmed from the response headers rather than the docs:
a `/players` response carries `x-ratelimit-remaining` and a `/matches` response carries no
rate-limit header at all.

That asymmetry shapes the code. A limiter applied to every call would make a sync a hundred
times slower for nothing, so `internal/connectors/pubg` consults a named predicate and paces
only the paths the publisher actually meters. Resolving a player costs; reading their matches
does not.

PUBG also does not send `Retry-After`. It sends `X-RateLimit-Reset`, an **absolute** UNIX
timestamp, so the back-off is computed against the clock with a ceiling in case the clocks
disagree.

**The publisher deletes matches after 14 days**, itself included. `pubg_matches` is therefore
the only place that history survives, which is the reason the table exists: anything not
collected within the fortnight is gone for everyone, forever. That is also why a failure that
can never succeed, such as a CHECK violation, is told apart from a passing one: retrying it
daily would burn the fortnight and lose the match anyway.

Finally, what counts as a match is an **accept list**, not a reject list. A survey of forty
real matches turned up a type absent from an earlier survey of six, so an unknown type is
dropped by default rather than stored by accident.

## Key data

`orgs`, `users`, `refresh_tokens` (device-bound), `device_pairings`, `invites`, `games`,
`linked_accounts`, `game_sessions`, `external_playtime`, `achievements`, `image_cache`.

## Security model

- Passwords: **Argon2id**. Login is timing-oracle-safe; weak/common passwords rejected (NIST).
- Tokens: **15-minute JWT** access + single-use, **device-bound refresh tokens** (rotated).
- Client linking: **OAuth device grant** - approved from the browser, never a password in the app.
- OAuth secrets at rest: **AES-256-GCM** (master key in env). *(Steam needs no per-user secret.)*
- HTTPS enforced (Caddy, or your reverse proxy). Rate limiting on `/auth`.
- Privacy: per-member toggle hides the current game from other members.

## The PlayStation bot

Sony publishes no API. KFIRE uses the endpoints of the PlayStation app, as ONE
bot account per instance (`internal/connectors/psn`, `internal/psnsync`). A
member types their online ID on the account page and adds the bot as a friend;
the poller accepts friend requests only from account ids linked to a KFIRE
member, and ignores any other.

**Why friendship.** With Sony's default privacy, played games, durations and
trophies are refused to a non-friend (403), and presence often is too. Measured
on three real members on 2026-09-30.

**The token.** The bot's NPSSO is the only secret: pasted by an admin in
Admin > PlayStation, proven against Sony before it is stored, sealed with
`KFIRE_MASTER_KEY`, never returned. Access tokens last 1 hour and are renewed
with a 10-day refresh token, then with the NPSSO. When Sony refuses the NPSSO
(`login_required`, code 4165) the bot is marked `needs_npsso`, the error is
logged once, the admin page turns red, and nothing hits Sony until a new NPSSO
is pasted. Sony announces 60 days for an NPSSO; the first one was revoked within
6 days (probably by a new sign-in on the bot account), so the admin page shows
its age.

**Every minute** (`KFIRE_PSN_POLL_INTERVAL`): accept linked members' requests,
read friends' presence in batches of 100, open or close `psn_api` sessions.
**Every 6 hours per member**: played games with their duration into
`external_playtime` (provider `psn`, baseline + delta like Steam), then the
earned trophies of the 10 most recently updated trophy lists.

**Catalog.** A PlayStation game has one concept id across PS4, PS5 and regions,
and many title ids. `games.psn_concept_id` ties a concept to a catalog game,
adopting the PC game of the same normalized name (trademark signs, platform and
language tails removed), so console hours add to the PC game. `psn_titles` maps
every title id to its game, since presence only names a title id. Trophy lists
never create a game: they join one by name or wait.

## The Nintendo bot

Like PlayStation, one bot account per instance that members befriend
(`internal/connectors/nintendo`, `internal/nintendosync`). A member types their
Switch friend code; the poller accepts friend requests only from linked
members, then reads every friend's presence in one call (`/v4/Friend/List`) and
each member's play history (`/v4/User/PlayLog/Show`, minutes per game, since the
first play). Nintendo has no achievements.

**The sidecar.** The Nintendo Switch Online app's API needs an "f" token computed
outside Nintendo, and encrypted requests. Both are provided by nxapi and its
public f-generation API, which the `nxapi` service of the compose files runs
(`deploy/nxapi`, pinned version). The Go server reaches it on the internal
network only, through `POST /api/znc/call`. When a Nintendo app update breaks
the pinned nxapi, bump the version in `deploy/nxapi/Dockerfile` and redeploy.

**Credentials.** Each instance registers its own OAuth client on
nxapi-auth.fancy.org.uk (scopes `ca:gf ca:er ca:dr`). nxapi-auth issues a shared
secret, not a client secret: the sidecar's entrypoint signs an HS256 client
assertion with it at every start (`KFIRE_NXAPI_CLIENT_ID`,
`KFIRE_NXAPI_SHARED_SECRET`). The bot's Nintendo session token is the instance's
other secret: an admin logs the bot in from Admin > Nintendo (PKCE flow of the
app, the npf link is pasted back), it is proven through the sidecar before being
stored sealed. `invalid_grant` marks the bot `needs_login` and the poller stops.

**Terms.** The nxapi API terms require a free service, an identifying
User-Agent, cached tokens, no automatic retry and a single automated request at
a time: the poller runs every call sequentially and never retries. The member
card says that presence and play history pass through nxapi.

**Catalog.** A Switch game is known by its eShop title id (Switch 2 editions have
their own). `nintendo_titles` maps it to a catalog game, adopting the PC game of
the same normalized name (`internal/gametitle`, shared with PlayStation: "for
Nintendo Switch", "Édition Essentielle"... are removed), so console hours add up
with PC hours. A member's friend code is shown on their profile to other members,
never through the public API.
