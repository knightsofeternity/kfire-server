-- 0013: add curated process names the Discord list misses, mirroring
-- extraExecutables in internal/games/discord.go. Palia's Steam build runs
-- PaliaClientSteam-Win64-Shipping.exe, while the list only carries the Epic
-- name, so Steam players were never detected.

BEGIN;

UPDATE games
SET executable_names = array_append(executable_names, 'paliaclientsteam-win64-shipping.exe')
WHERE slug = 'palia'
  AND NOT ('paliaclientsteam-win64-shipping.exe' = ANY(executable_names));

COMMIT;
