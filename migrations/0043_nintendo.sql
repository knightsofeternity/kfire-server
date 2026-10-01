-- 0043: the Nintendo Switch connector.
--
-- One bot account per instance, befriended by members, read through an nxapi
-- sidecar. Its Nintendo session token is the only secret: sealed with the
-- master key, never returned by the API. A pending login keeps its PKCE
-- verifier sealed too, for the few minutes an admin needs to sign in.
--
-- A Switch game is known by its eShop title id (Switch 1 "0100…" and Switch 2
-- editions "0400…" are distinct). nintendo_titles maps each to a catalog game,
-- which may be the PC game of the same name: console hours then add up.

BEGIN;

ALTER TABLE linked_accounts DROP CONSTRAINT IF EXISTS linked_accounts_provider_check;
ALTER TABLE linked_accounts ADD CONSTRAINT linked_accounts_provider_check
    CHECK (provider IN ('steam', 'battlenet', 'riot', 'epic', 'xbox', 'psn', 'pubg', 'nintendo'));

ALTER TABLE external_playtime DROP CONSTRAINT IF EXISTS external_playtime_provider_check;
ALTER TABLE external_playtime ADD CONSTRAINT external_playtime_provider_check
    CHECK (provider IN ('steam', 'xbox', 'psn', 'nintendo'));

ALTER TABLE game_sessions DROP CONSTRAINT IF EXISTS game_sessions_source_check;
ALTER TABLE game_sessions ADD CONSTRAINT game_sessions_source_check
    CHECK (source IN ('client', 'steam_api', 'xbox_api', 'psn_api', 'nintendo_api'));

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_platform_check;
ALTER TABLE games ADD CONSTRAINT games_platform_check
    CHECK (platform IN ('pc', 'xbox', 'playstation', 'nintendo'));

CREATE TABLE nintendo_bot (
    id                 int         PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    session_enc        bytea,
    nickname           text,
    nsa_id             text,
    friend_code        text,
    status             text        NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'needs_login')),
    last_error         text,
    session_set_at     timestamptz,
    last_ok_at         timestamptz,
    login_state        text,
    login_verifier_enc bytea,
    login_started_at   timestamptz
);

-- A member's friend code, kept to show it on their profile: on a Switch,
-- friends add each other by code, not by name.
CREATE TABLE nintendo_accounts (
    user_id     uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    friend_code text NOT NULL CHECK (friend_code ~ '^[0-9]{4}-[0-9]{4}-[0-9]{4}$')
);

CREATE TABLE nintendo_titles (
    title_id text PRIMARY KEY CHECK (title_id ~ '^[0-9a-f]{16}$'),
    game_id  uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE
);

CREATE TABLE nintendo_library_sync (
    user_id   uuid        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    synced_at timestamptz NOT NULL DEFAULT now()
);

COMMIT;
