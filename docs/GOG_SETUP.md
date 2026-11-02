# GOG connector

Members link their own GOG account; KFIRE then imports, every 6 hours, their
GOG games, the playtime GOG Galaxy counted for each of them and the
achievements they unlocked. Galaxy counts only what was played through Galaxy
(or a launcher that reports to it, such as Heroic), and only games built with
Galaxy have achievements: the rest of a GOG game's hours come from the desktop client's
sessions, like any PC game. Games Galaxy imported from other stores (Steam,
Epic, Xbox...) are left out: KFIRE gets them from their own connectors. There is
no live presence and nothing about a member's friends.

## Credentials

GOG offers no API for libraries or playtime. KFIRE uses the services the GOG
Galaxy client itself uses, with Galaxy's client credentials, exactly like the
open-source launchers Heroic and Comet. They are public (Heroic's source,
repository `Heroic-Games-Launcher/HeroicGamesLauncher`, and `lgogdownloader`),
and they are **not** shipped with KFIRE: each instance decides whether to use
them.

    KFIRE_GOG_CLIENT_ID=<Galaxy client id>
    KFIRE_GOG_CLIENT_SECRET=<Galaxy client secret>

Both empty (the default) turns the connector off: no account card, no call to
GOG. This is not an access GOG granted to KFIRE; if GOG changes these
credentials, the server logs `gog: Galaxy client rejected` and you update the
two values, without any code change.

## Linking, as a member

1. Account page, GOG card, **Sign in to GOG**.
2. After signing in, GOG shows an almost blank page whose address starts with
   `https://embed.gog.com/on_login_success` and holds `code=`. Paste that whole
   address in the card within a few minutes (the code is single-use).
3. The first import starts at once. The sync renews the sign-in by itself; when
   GOG refuses it, the card asks to sign in again.

Unlinking stops the sync and keeps the hours already imported.
