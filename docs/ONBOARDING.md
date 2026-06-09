# Player onboarding

The journey for a new member, once the server is set up.

```
1. Invite ──▶ 2. Create account ──▶ 3. Download the app ──▶ 4. Link the device ──▶ 5. Playing
   (admin)      (web, no password         (per-platform          (browser approval)     (tray)
                 in the app later)         installer)
```

### 1. Invitation

An admin opens **Admin → Invite a member**, creates an invite (optionally noting who it's
for), and shares the generated link: `https://<server>/?invite=CODE`. Invites are single-use
and expire after 14 days. With `KFIRE_OPEN_REGISTRATION=false`, only invited people can join.

### 2. Create the account

The member opens the link, fills in a display name, email and password (12+ chars; the
strength meter guides them), and lands in the web app.

### 3. Download the desktop app

From **Get the app**, the page detects their OS and offers the matching installer from the
latest [GitHub release](https://github.com/knightsofeternity/kfire-client/releases) - Windows
`.msi`/`.exe`, macOS `.dmg`, Linux `.AppImage`/`.deb`.

### 4. Link the device (browser approval, no password)

On first launch the member enters the **server address**. The app:

1. registers a pairing and gets a short code (e.g. `WHAU-55AP`),
2. opens the browser at `https://<server>/link?code=…`,
3. the member - already signed in - sees *“Link this device: Gaming PC (Windows)?”* and
   approves,
4. the app polls, receives **device-bound tokens**, and connects.

This is the OAuth **device authorization grant**: the password is never typed into the app,
and the link is bound to that specific device. Tokens can be revoked (Unlink) anytime.

### 5. Playing

The client lives in the tray, scans running processes every few seconds, and reports
`game_started` / `game_stopped` over WebSocket. The member appears live on the dashboard, and
their playtime feeds profiles, per-game leaderboards and stats. Linking Steam (in **Account →
Connected accounts**) additionally imports library playtime and achievements.
