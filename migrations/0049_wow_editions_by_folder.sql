-- 0049: tell World of Warcraft editions apart by their folder (November 2026).
--
-- Every WoW edition runs a binary named Wow.exe; only the edition folder
-- differs (World of Warcraft/_retail_/Wow.exe, .../_forever_/Wow.exe). The
-- catalog matched retail on the bare wow.exe and Forever on wowb.exe, which is
-- the BETA client's name. Result, proven on a member's PC: a WoW Forever
-- session was reported as World of Warcraft (retail), and Forever was never
-- detected at all.
--
-- discord.go now drops the bare names and seeds the qualified patterns
-- "_retail_/wow.exe" and "_forever_/wow.exe"; this migration patches the live
-- rows so detection is right as soon as the server is deployed, without
-- waiting for the weekly catalog re-sync. Clients >= 0.4.0 match patterns.

BEGIN;

UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_remove(executable_names, 'wow.exe') || ARRAY['_retail_/wow.exe']) AS e
    )
WHERE slug = 'world-of-warcraft'
  AND (executable_names @> ARRAY['wow.exe'] OR NOT executable_names @> ARRAY['_retail_/wow.exe']);

UPDATE games
SET executable_names = (
        SELECT array_agg(DISTINCT e)
        FROM unnest(array_remove(executable_names, 'wowb.exe') || ARRAY['_forever_/wow.exe']) AS e
    )
WHERE slug = 'world-of-warcraft-forever'
  AND (executable_names @> ARRAY['wowb.exe'] OR NOT executable_names @> ARRAY['_forever_/wow.exe']);

COMMIT;
