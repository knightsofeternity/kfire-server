-- 0017: Battle.net WoW profile enrichment. Persist the OAuth scopes granted at
-- link time (so the SPA can prompt members linked before profile scopes existed
-- to reconnect), and store each member's WoW characters per catalog game
-- (retail vs classic distinguished by game_id).

BEGIN;

ALTER TABLE linked_accounts
    ADD COLUMN scopes text[] NOT NULL DEFAULT '{}';

CREATE TABLE bnet_wow_characters (
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id        uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    region         text NOT NULL,
    realm_slug     text NOT NULL,
    name           text NOT NULL,
    faction        text,
    race           text,
    class          text,
    level          int  NOT NULL DEFAULT 0,
    item_level     int  NOT NULL DEFAULT 0,
    mythic_rating  numeric,
    raid_summary   jsonb,
    last_synced_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, region, realm_slug, name)
);

CREATE INDEX bnet_wow_characters_game_ilvl_idx
    ON bnet_wow_characters (game_id, item_level DESC);

COMMIT;
