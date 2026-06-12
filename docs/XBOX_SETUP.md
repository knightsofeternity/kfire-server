# Xbox connector setup (OpenXBL)

The Xbox connector brings **console presence** into KFIRE: a member who links
their Xbox and plays on console shows up as `in_game` on the dashboard, players
list and game pages — even though no desktop client can run on a console. It
uses [OpenXBL](https://xbl.io) (a third-party Xbox Live gateway) for OAuth and
presence reads.

The connector is **disabled** (the link button returns HTTP 501) until
`KFIRE_XBL_APP_KEY` is set. Setting it up requires both an **OpenXBL account**
and a **Microsoft Azure (Entra ID) app registration** — OpenXBL drives the
Microsoft login with your own Azure app.

## One-time setup (admin)

1. **OpenXBL account.** Sign in at <https://xbl.io> with a Microsoft account that
   has an Xbox profile (your gamertag must show; the account's API key returns an
   empty profile otherwise). Your personal **API Key** is on the dashboard.

2. **Create an OAuth app on OpenXBL** (Apps → Create). It walks you through an
   "Azure Setup" step.

3. **Azure app registration** (portal.azure.com → App registrations → New):
   - **Name**: e.g. `KFIRE - Knights of Eternity`.
   - **Supported account types**: **Personal Microsoft accounts only** (or
     "any org directory **and** personal Microsoft accounts"). **NOT** "single
     tenant / Default Directory" — Xbox accounts are personal Microsoft accounts,
     and a single-tenant app rejects them with *Unauthorized*. If the option is
     greyed out, edit the **Manifest**: `"signInAudience":
     "AzureADandPersonalMicrosoftAccount"`.
   - **Redirect URI** (platform **Web**): **`https://api.xbl.io/app/callback`**
     — this is *OpenXBL's* callback, not ours. OpenXBL tokenizes, then redirects
     to our app.
   - Register, then copy the **Application (client) ID**.
   - **Certificates & secrets → New client secret** → copy the **Value**
     immediately (the table view truncates it; the Value is only fully shown once,
     at creation — copy the Value, not the Secret ID).

4. **Finish the OpenXBL app**: paste the Azure **client ID** + **client secret**,
   and set the app's redirect to **our** callback:
   `https://kfire.guilde-ke.fr/api/v1/connect/xbox/callback`. OpenXBL's dashboard
   "Test" step is authenticated by your logged-in browser session — run it **from
   the site** (clicking through), not via curl/Postman, or it returns
   `401 {"error":"Unauthorized"}`. You can finish the app and grab the key even if
   the test is finicky.

5. **Copy the OpenXBL Public Key** (on <https://xbl.io/apps>). This is
   `KFIRE_XBL_APP_KEY` — distinct from the Azure client ID and from your personal
   API key.

6. **Configure the server**:
   - Add `KFIRE_XBL_APP_KEY=<public key>` to `~/kfire-server/.env`.
   - The proxied compose passes env vars **explicitly**, so the var is also wired
     in `docker-compose.proxied.yml` under the server `environment:` block
     (`KFIRE_XBL_APP_KEY: ${KFIRE_XBL_APP_KEY:-}`). New `KFIRE_*` vars must be
     added there or they never reach the container (same trap as
     `KFIRE_OPEN_REGISTRATION`).
   - Recreate: `docker compose -f docker-compose.proxied.yml up -d --build --force-recreate server`.
   - Confirm in the logs: `xboxsync: poller started`.

The Azure **client ID/secret live in OpenXBL only** (it does the Microsoft login
on your behalf). KFIRE only needs the OpenXBL **Public Key**.

## Environment variables

| Var | Purpose |
| --- | --- |
| `KFIRE_XBL_APP_KEY` | OpenXBL Public Key. Empty = connector disabled (501). |
| `KFIRE_XBOX_POLL_INTERVAL` | Presence poll cadence (default `2m`, min `30s`). |
| `KFIRE_XBL_API_BASE` | Override the OpenXBL base URL (tests only). |

## How it works

- **Authorize**: `GET https://api.xbl.io/app/auth/{publicKey}` → Microsoft login
  (your Azure app, personal accounts) → OpenXBL redirects to our callback with
  `?code=`.
- **Identify the user**: our `/connect/xbox` start sets a signed, short-lived
  cookie with the member's KFIRE user id (OpenXBL does not echo an OAuth `state`).
- **Claim**: `POST https://api.xbl.io/app/claim` with `{code, app_key}` returns
  the member's secret key **plus** `xuid` and `gamertag`. We store the encrypted
  key (`linked_accounts`, AES-256-GCM) and the gamertag.
- **Presence poller** (`internal/xboxsync`): every `KFIRE_XBOX_POLL_INTERVAL`,
  for each linked member, `GET https://xbl.io/api/v2/{xuid}/presence` with the
  member's key as `X-Authorization`, and reconciles the currently-playing title
  into an open `game_sessions(source='xbox_api')` row, broadcasting presence.
- **Hosts**: auth + claim are on `api.xbl.io`; data calls (account, presence) are
  on `xbl.io`.

## Pitfalls (learned the hard way)

- **Azure account type** must include personal Microsoft accounts, or every link
  fails with *Unauthorized*.
- **Two redirect URIs**: Azure's is `https://api.xbl.io/app/callback`; the
  OpenXBL app's is our `/api/v1/connect/xbox/callback`.
- **Client secret**: copy the **Value** at creation (not the Secret ID; the table
  masks it afterwards).
- **OpenXBL `/apps/test`** is browser-session authenticated — run it from the
  logged-in site, not curl.
- **Auth/claim host is `api.xbl.io`**, data host is `xbl.io` — mixing them 404s.
- **The claim is slow** (~10-30s; even a bogus claim ~9s) — we use a dedicated
  60s timeout for it, and the account page warns members the link takes ~30s.
- **`/api/v2/account` returns an empty profile** for app-scoped member keys, so
  identity comes from the **claim response** (`xuid`/`gamertag`), not a follow-up
  account call.
- **Rate limit**: free tier is **150 req/h** (`x-ratelimit-limit` header) — about
  5 members at a 2-minute poll. Relax `KFIRE_XBOX_POLL_INTERVAL` or upgrade the
  OpenXBL plan for more members.
