-- 0046: make WARDOGS detectable (reported by members in October 2026: "hours
-- counted, but not on our profiles nor live").
--
-- The live row (steam 1867240) was created by a member's Steam library import,
-- which inserts owned games with NO executables. Its hours therefore arrive
-- through the Steam playtime import only, and the client can never open a
-- session, so the game never shows live. Discord carries it with an empty
-- executables array too (app 1511573843330732122), so normalize() had nothing
-- to seed. discord.go now adds curated executables, taken from the app's Steam
-- launch configuration; this migration patches the live catalog so detection
-- works right after deploy. Mirrors 0030.
--
-- Not to be confused with "WarDogs: Red's Return" (Discord 1442735719008309339),
-- a different game.
--
-- Adopting the Discord app id matters: UpsertGames is keyed on discord_app_id,
-- so without it a later re-sync would INSERT a second row (suffixed slug) and
-- split the game's history in two.

BEGIN;

UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(executable_names || ARRAY['wardogslauncher-shipping.exe', 'wardogsclient-win64-shipping.exe']) AS e
    ),
    discord_app_id = COALESCE(discord_app_id, '1511573843330732122')
WHERE (steam_app_id = '1867240' OR discord_app_id = '1511573843330732122')
  AND NOT (executable_names @> ARRAY['wardogslauncher-shipping.exe', 'wardogsclient-win64-shipping.exe']);

INSERT INTO games (name, slug, executable_names, platform, discord_app_id, steam_app_id)
SELECT 'WARDOGS', 'wardogs',
       ARRAY['wardogslauncher-shipping.exe', 'wardogsclient-win64-shipping.exe'], 'pc',
       '1511573843330732122', '1867240'
WHERE NOT EXISTS (
    SELECT 1 FROM games
    WHERE discord_app_id = '1511573843330732122'
       OR steam_app_id = '1867240'
       OR slug = 'wardogs'
);

COMMIT;
