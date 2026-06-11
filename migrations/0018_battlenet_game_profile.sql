-- 0018: generic per-member, per-game Battle.net profile cache for games whose
-- stats don't warrant a typed table (Diablo III, StarCraft II). Shape of `data`
-- is owned by the connector + SPA. Throttling reuses bnet_wow_sync.

BEGIN;

CREATE TABLE bnet_game_profile (
    user_id        uuid  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id        uuid  NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    data           jsonb NOT NULL,
    last_synced_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game_id)
);

COMMIT;
