-- 0044: World of Warcraft /played, read by the KFire addon.
--
-- Blizzard publishes no playtime. The KFire addon records each character's
-- /played in game; the desktop client sends the list as a match_result whose
-- game_slug is the edition (retail, classic, forever, ascension). One row per
-- member, edition and character; a row never goes back in time.

BEGIN;

CREATE TABLE wow_played (
    user_id        uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id        uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    region         text        NOT NULL CHECK (region IN ('us', 'eu', 'kr', 'tw', 'cn', 'unknown')),
    realm_norm     text        NOT NULL CHECK (length(realm_norm) BETWEEN 1 AND 64),
    name           text        NOT NULL CHECK (length(name) BETWEEN 2 AND 24),
    realm          text        NOT NULL CHECK (length(realm) BETWEEN 1 AND 64),
    played_seconds bigint      NOT NULL CHECK (played_seconds >= 0),
    level          int,
    class          text,
    recorded_at    timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game_id, region, realm_norm, name)
);

ALTER TABLE external_playtime DROP CONSTRAINT IF EXISTS external_playtime_provider_check;
ALTER TABLE external_playtime ADD CONSTRAINT external_playtime_provider_check
    CHECK (provider IN ('steam', 'xbox', 'psn', 'nintendo', 'wow_addon'));

COMMIT;
