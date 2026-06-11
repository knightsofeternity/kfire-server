-- 0016: make Vampire Crawlers detectable. Discord's detectable list carries the
-- game (discord 1477144137097547943, steam 3265700) with an EMPTY executables
-- array, so normalize() dropped it from the catalog and the client could never
-- match it. discord.go now adds the curated "vampire crawlers.exe" for fresh
-- seeds; this migration patches the live catalog so detection works on deploy
-- without waiting for a full re-seed, mirroring 0013 (curated executables).

BEGIN;

-- Case 1: a row already exists - e.g. created by a member's Steam library
-- import, which inserts owned games with no executables. Append the exe and
-- adopt the Discord app id so a later re-seed (keyed on discord_app_id) updates
-- THIS row instead of inserting a duplicate.
UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_append(executable_names, 'vampire crawlers.exe')) AS e
    ),
    discord_app_id = COALESCE(discord_app_id, '1477144137097547943')
WHERE (steam_app_id = '3265700' OR discord_app_id = '1477144137097547943')
  AND NOT ('vampire crawlers.exe' = ANY(executable_names));

-- Case 2: no row exists yet. Create it so the game is detectable immediately.
INSERT INTO games (name, slug, executable_names, platform, discord_app_id, steam_app_id)
SELECT 'Vampire Crawlers: The Turbo Wildcard from Vampire Survivors',
       'vampire-crawlers-the-turbo-wildcard-from-vampire-survivors',
       ARRAY['vampire crawlers.exe'], 'pc',
       '1477144137097547943', '3265700'
WHERE NOT EXISTS (
    SELECT 1 FROM games
    WHERE discord_app_id = '1477144137097547943' OR steam_app_id = '3265700'
);

COMMIT;
