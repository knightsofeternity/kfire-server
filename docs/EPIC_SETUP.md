# Epic Games connector

Members link their own Epic account; KFIRE then imports, every 6 hours, their
Epic library and the playtime Epic counts. Epic counts playtime for some games
only (Fortnite, mostly): the rest of an Epic game's hours come from the
desktop client's sessions, like any PC game. There is no live presence and
nothing about a member's friends.

## Credentials

Epic offers no API for libraries or playtime. KFIRE uses the services the Epic
launcher itself uses, with the launcher's client credentials, exactly like the
open-source launchers Legendary and Heroic. They are public, published in
Legendary's source (`legendary/api/egs.py`, repository `derrod/legendary`), and
they are **not** shipped with KFIRE: each instance decides whether to use them.

    KFIRE_EPIC_CLIENT_ID=<from Legendary>
    KFIRE_EPIC_CLIENT_SECRET=<from Legendary>

Both empty (the default) turns the connector off: no account card, no call to
Epic. This is not an access Epic granted to KFIRE; if Epic changes these
credentials, the server logs `epic: launcher client rejected` and you update the
two values from Legendary, without any code change.

## Linking, as a member

1. Account page, Epic Games card, **Sign in to Epic**.
2. After signing in, Epic shows a short JSON containing `authorizationCode`.
   Paste all of it in the card within a few minutes (the code is single-use).
3. The first import starts at once. The link lasts a year; when Epic refuses it,
   the card asks to sign in again.

Unlinking stops the sync and keeps the hours already imported.
