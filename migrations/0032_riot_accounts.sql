-- 0032: Riot connector. Two tables, both keyed on the member.
--
-- riot_accounts carries routing only: which platform host serves this member's
-- League data, and which match cluster serves their match history. The identity
-- (PUUID, Riot ID) lives in linked_accounts, whose provider CHECK already
-- allows 'riot'. No OAuth token is stored: RSO proves ownership once at link
-- time and every later read uses the server's API key.
--
-- riot_game_profile mirrors bnet_game_profile exactly: one opaque JSON blob per
-- member per catalog game, shaped by the connector and read by the SPA. It also
-- carries the refresh throttle, so no separate sync table is needed.

BEGIN;

CREATE TABLE riot_accounts (
    user_id       uuid        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    platform      text        NOT NULL,
    match_cluster text        NOT NULL,
    region_source text        NOT NULL DEFAULT 'auto'
                  CHECK (region_source IN ('auto', 'manual')),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE riot_game_profile (
    user_id        uuid  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id        uuid  NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    data           jsonb NOT NULL,
    last_synced_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game_id)
);

CREATE INDEX riot_game_profile_game_idx ON riot_game_profile (game_id);

COMMIT;
