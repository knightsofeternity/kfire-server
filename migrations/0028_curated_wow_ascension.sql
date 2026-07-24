-- 0028: make WoW Ascension (Project Ascension, private WoW server) detectable as
-- its own catalog game. Ascension is NOT in Discord's detectable list, so there
-- is no row to match. We insert a curated row with discord_app_id NULL, which
-- UpsertGames (keyed on discord_app_id) never touches or deletes, so it survives
-- an admin "sync games" re-seed. We match ONLY 'ascension.exe' (the game client =
-- real presence). 'Wow.exe' is intentionally excluded (generic, collides with
-- retail WoW and other private servers); the launcher/services are excluded too
-- (open != in game). Mirrors 0016 (curated Vampire Crawlers), Case 2.
BEGIN;

INSERT INTO games (name, slug, executable_names, platform, icon_url, discord_app_id, steam_app_id)
SELECT 'WoW Ascension',
       'wow-ascension',
       ARRAY['ascension.exe'],
       'pc',
       'https://kfire.guilde-ke.fr/game-icons/wow-ascension.png',
       NULL,
       NULL
WHERE NOT EXISTS (SELECT 1 FROM games WHERE slug = 'wow-ascension');

COMMIT;
