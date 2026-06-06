-- 0001_init.sql — KFIRE initial schema
--
-- Conventions:
--   * UUID primary keys (gen_random_uuid, pgcrypto is built-in since PG 13)
--   * timestamptz everywhere, UTC
--   * Secrets are NEVER stored in clear: password_hash is Argon2id,
--     OAuth tokens are AES-256-GCM ciphertexts (bytea), key in env.

BEGIN;

-- One server instance = one organization, but the table exists from day one
-- so the schema never needs a painful retrofit.
CREATE TABLE orgs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    slug       text        NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    username      text        NOT NULL,
    email         text        NOT NULL,
    -- Argon2id encoded hash ($argon2id$v=19$...), never bcrypt.
    password_hash text        NOT NULL,
    role          text        NOT NULL DEFAULT 'member'
                  CHECK (role IN ('admin', 'member')),
    avatar_url    text,
    banned_at     timestamptz,          -- soft ban; NULL = active
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, username),
    UNIQUE (email)
);

-- Device-bound refresh tokens (single-use, rotated on refresh).
CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id   uuid        NOT NULL,
    device_name text        NOT NULL DEFAULT '',
    platform    text        NOT NULL DEFAULT ''
                CHECK (platform IN ('', 'windows', 'macos', 'linux', 'web')),
    -- SHA-256 of the opaque token; the clear value never touches the DB.
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, device_id)
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- Games catalog. Seeded from the Discord "detectable games" list; admins can
-- add custom entries.
CREATE TABLE games (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text   NOT NULL,
    slug             text   NOT NULL UNIQUE,
    -- Process names the client matches against (e.g. {'cs2.exe','cs2'}).
    executable_names text[] NOT NULL DEFAULT '{}',
    platform         text   NOT NULL DEFAULT 'pc'
                     CHECK (platform IN ('pc', 'xbox', 'playstation')),
    icon_url         text,
    created_at       timestamptz NOT NULL DEFAULT now()
);

-- Fast lookup of a process name across all games.
CREATE INDEX games_executable_names_idx ON games USING gin (executable_names);

-- Linked external platform accounts (Steam, Battle.net, ...). OAuth tokens are
-- encrypted application-side with AES-256-GCM before insertion.
CREATE TABLE linked_accounts (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          text        NOT NULL
                      CHECK (provider IN ('steam', 'battlenet', 'riot', 'epic', 'xbox', 'psn')),
    provider_user_id  text        NOT NULL,
    display_name      text,
    access_token_enc  bytea,                -- AES-256-GCM(nonce || ciphertext || tag)
    refresh_token_enc bytea,
    token_expires_at  timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider),
    UNIQUE (provider, provider_user_id)
);

-- Game sessions, from the desktop client or from platform API polling.
CREATE TABLE game_sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id    uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    source     text        NOT NULL DEFAULT 'client'
               CHECK (source IN ('client', 'steam_api', 'xbox_api', 'psn_api')),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at   timestamptz,                 -- NULL while the session is open
    duration_seconds integer GENERATED ALWAYS AS
               (CASE WHEN ended_at IS NULL THEN NULL
                     ELSE GREATEST(0, EXTRACT(epoch FROM ended_at - started_at))::integer
                END) STORED,
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);

-- History queries: "sessions of user X, most recent first".
CREATE INDEX game_sessions_user_started_idx ON game_sessions (user_id, started_at DESC);
-- Stats queries: "hours per game".
CREATE INDEX game_sessions_game_idx ON game_sessions (game_id);
-- Live presence: open sessions only — tiny partial index.
CREATE INDEX game_sessions_open_idx ON game_sessions (user_id) WHERE ended_at IS NULL;
-- At most one open session per user and game (reconnect dedup).
CREATE UNIQUE INDEX game_sessions_open_unique ON game_sessions (user_id, game_id)
    WHERE ended_at IS NULL;

COMMIT;
