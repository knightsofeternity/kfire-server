-- 0030: make four games detectable that members reported as never showing up
-- (July 2026 beta feedback).
--
-- Three of them (Battlesector, Olden Era, Subnautica 2) exist in the catalog
-- only because a member's Steam library import created the row, which inserts
-- owned games with NO executables. Discord carries all three with an empty
-- executables array too, so normalize() had nothing to seed. discord.go now
-- adds a curated executable (taken from each app's Steam launch config) for
-- fresh seeds; this migration patches the live catalog so detection works right
-- after deploy, without waiting for a re-sync. Mirrors 0016 (Vampire Crawlers).
--
-- The fourth (DRAGON BALL GEKISHIN SQUADRA) ships a single binary literally
-- named game.exe, too generic to match on its own. Discord qualifies it with
-- the install directory and the client now matches such entries as a suffix of
-- the running process path, so the qualified pattern is what makes it work.
--
-- Adopting the Discord app id on the Steam-created rows matters: UpsertGames is
-- keyed on discord_app_id, so without it a later re-sync would INSERT a second
-- row for the same game (suffixed slug) and split its history in two.

BEGIN;

-- Warhammer 40,000: Battlesector (steam 1295500, discord 1439432142466584758).
-- Its Steam launch option is the generic Launcher.exe; the real binary is
-- "Warhammer 40K Battlesector.exe".
UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_append(executable_names, 'warhammer 40k battlesector.exe')) AS e
    ),
    discord_app_id = COALESCE(discord_app_id, '1439432142466584758')
WHERE (steam_app_id = '1295500' OR discord_app_id = '1439432142466584758')
  AND NOT ('warhammer 40k battlesector.exe' = ANY(executable_names));

INSERT INTO games (name, slug, executable_names, platform, discord_app_id, steam_app_id)
SELECT 'Warhammer 40,000: Battlesector', 'warhammer-40-000-battlesector',
       ARRAY['warhammer 40k battlesector.exe'], 'pc',
       '1439432142466584758', '1295500'
WHERE NOT EXISTS (
    SELECT 1 FROM games
    WHERE discord_app_id = '1439432142466584758'
       OR steam_app_id = '1295500'
       OR slug = 'warhammer-40-000-battlesector'
);

-- Heroes of Might and Magic: Olden Era (steam 3105440, discord
-- 1428182804918698115). Discord names it "Might & Magic", Steam "Might and
-- Magic", hence the two different slugs; the live row keeps its own.
UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_append(executable_names, 'heroesoldenera.exe')) AS e
    ),
    discord_app_id = COALESCE(discord_app_id, '1428182804918698115')
WHERE (steam_app_id = '3105440' OR discord_app_id = '1428182804918698115')
  AND NOT ('heroesoldenera.exe' = ANY(executable_names));

INSERT INTO games (name, slug, executable_names, platform, discord_app_id, steam_app_id)
SELECT 'Heroes of Might & Magic: Olden Era', 'heroes-of-might-magic-olden-era',
       ARRAY['heroesoldenera.exe'], 'pc',
       '1428182804918698115', '3105440'
WHERE NOT EXISTS (
    SELECT 1 FROM games
    WHERE discord_app_id = '1428182804918698115'
       OR steam_app_id = '3105440'
       OR slug = 'heroes-of-might-magic-olden-era'
);

-- Subnautica 2 (steam 1962700, discord 1505320535268261888).
UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_append(executable_names, 'subnautica2.exe')) AS e
    ),
    discord_app_id = COALESCE(discord_app_id, '1505320535268261888')
WHERE (steam_app_id = '1962700' OR discord_app_id = '1505320535268261888')
  AND NOT ('subnautica2.exe' = ANY(executable_names));

INSERT INTO games (name, slug, executable_names, platform, discord_app_id, steam_app_id)
SELECT 'Subnautica 2', 'subnautica-2',
       ARRAY['subnautica2.exe'], 'pc',
       '1505320535268261888', '1962700'
WHERE NOT EXISTS (
    SELECT 1 FROM games
    WHERE discord_app_id = '1505320535268261888'
       OR steam_app_id = '1962700'
       OR slug = 'subnautica-2'
);

-- DRAGON BALL GEKISHIN SQUADRA (steam 2072560, discord 1415260575188910110):
-- qualified path pattern, matched by clients from v0.4.0 on. Older clients
-- simply never match it, exactly as today.
UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_append(executable_names, 'dragon ball gekishin squadra/game.exe')) AS e
    ),
    discord_app_id = COALESCE(discord_app_id, '1415260575188910110')
WHERE (steam_app_id = '2072560' OR discord_app_id = '1415260575188910110')
  AND NOT ('dragon ball gekishin squadra/game.exe' = ANY(executable_names));

COMMIT;
