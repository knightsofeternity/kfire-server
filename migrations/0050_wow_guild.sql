-- 0050: WoW Forever guild, read by the KFireGuilde addon (lot 5).
--
-- The desktop client adds an optional `guild` object to the Forever /played
-- match_result (contract: kfire-client wow-addon/docs/kfire-guilde-liaison.md).
-- A guild is identified per edition by lower(guild_name) + realm_norm. Every
-- member's addon reports the same guild: the roster is replaced by a newer
-- snapshot, each profile and profession keeps its newest updated_at, and the
-- hourly presence merges by keeping the larger count of each hour.
--
-- Characters are stored as the addon names them, "Name-Realm". The link to a
-- KFIRE member is not stored: it is read from wow_played at query time, so it
-- follows the /played as members come and go.

BEGIN;

CREATE TABLE wow_guilds (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id     uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    guild_key   text        NOT NULL CHECK (length(guild_key) BETWEEN 1 AND 64 AND guild_key = lower(guild_key)),
    realm_norm  text        NOT NULL CHECK (length(realm_norm) BETWEEN 1 AND 64),
    name        text        NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    snapshot_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, guild_key, realm_norm)
);

-- The latest roster snapshot, replaced whole by a newer one.
CREATE TABLE wow_guild_roster (
    guild_id     uuid        NOT NULL REFERENCES wow_guilds(id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (length(name) BETWEEN 4 AND 89),
    level        int         CHECK (level BETWEEN 1 AND 200),
    class        text        CHECK (length(class) BETWEEN 1 AND 32),
    rank         text        CHECK (length(rank) BETWEEN 1 AND 64),
    rank_index   int         CHECK (rank_index BETWEEN 0 AND 255),
    last_seen_at timestamptz,
    PRIMARY KEY (guild_id, name)
);

-- A character's addon profile. reported_by is who sent the newest version; a
-- member who leaves KFIRE does not take the guild's profiles with them.
CREATE TABLE wow_guild_members (
    guild_id      uuid        NOT NULL REFERENCES wow_guilds(id) ON DELETE CASCADE,
    name          text        NOT NULL CHECK (length(name) BETWEEN 4 AND 89),
    pseudo        text        CHECK (length(pseudo) BETWEEN 1 AND 32),
    main          text        CHECK (length(main) BETWEEN 4 AND 89),
    role          text        CHECK (role IN ('T', 'H', 'D')),
    addon_version text        CHECK (length(addon_version) BETWEEN 1 AND 32),
    updated_at    timestamptz NOT NULL,
    reported_by   uuid        REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (guild_id, name)
);

-- One profession of one character, with the recipes it knows (item ids, or
-- "s" + spell id).
CREATE TABLE wow_guild_professions (
    guild_id    uuid        NOT NULL REFERENCES wow_guilds(id) ON DELETE CASCADE,
    name        text        NOT NULL CHECK (length(name) BETWEEN 4 AND 89),
    profession  text        NOT NULL CHECK (length(profession) BETWEEN 1 AND 64),
    level       int         CHECK (level BETWEEN 0 AND 1000),
    max_level   int         CHECK (max_level BETWEEN 0 AND 1000),
    recipes     text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(recipes) <= 1000),
    updated_at  timestamptz NOT NULL,
    reported_by uuid        REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (guild_id, name, profession)
);

-- Item and spell names, per edition: the same id names the same thing in every
-- guild of the edition. The last reported name wins.
CREATE TABLE wow_item_names (
    game_id    uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    item_id    text        NOT NULL CHECK (item_id ~ '^s?[0-9]{1,10}$'),
    name       text        NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, item_id)
);

-- Members seen online per hour of the server's day, NULL when nobody was
-- watching that hour. Merged by the larger count of each hour.
CREATE TABLE wow_guild_presence (
    guild_id   uuid        NOT NULL REFERENCES wow_guilds(id) ON DELETE CASCADE,
    day        date        NOT NULL,
    hours      int[]       NOT NULL CHECK (cardinality(hours) = 24 AND 0 <= ALL (hours) AND 1000 >= ALL (hours)),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (guild_id, day)
);

COMMIT;
