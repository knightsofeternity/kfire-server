-- 0005_steam_library.sql - Steam library & achievement import.

BEGIN;

-- Steam AppID for catalog games (sourced from Discord's third_party_skus).
-- Lets us match a member's owned Steam games to the catalog.
ALTER TABLE games ADD COLUMN steam_app_id text;
CREATE INDEX games_steam_app_id_idx ON games (steam_app_id) WHERE steam_app_id IS NOT NULL;

-- Lifetime playtime imported from a platform (Steam: playtime_forever).
-- One row per (user, provider, game); re-synced in place.
CREATE TABLE external_playtime (
    user_id        uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider       text        NOT NULL
                   CHECK (provider IN ('steam', 'xbox', 'psn')),
    game_id        uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    total_seconds  bigint      NOT NULL DEFAULT 0,
    last_synced_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, provider, game_id)
);

-- Unlocked achievements imported from a platform.
CREATE TABLE achievements (
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     text        NOT NULL
                 CHECK (provider IN ('steam', 'xbox', 'psn')),
    game_id      uuid        REFERENCES games(id) ON DELETE SET NULL,
    api_name     text        NOT NULL,          -- platform achievement id
    display_name text,
    icon_url     text,
    unlocked_at  timestamptz NOT NULL,
    PRIMARY KEY (user_id, provider, game_id, api_name)
);

CREATE INDEX achievements_user_unlocked_idx ON achievements (user_id, unlocked_at DESC);

COMMIT;
