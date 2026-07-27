-- 0029: Discord lists 'ascension.exe' under "Ascension: Deckbuilding Game", but
-- that binary is the client of WoW Ascension (Project Ascension, the private WoW
-- server) added as a curated entry in 0028. Both rows carried the same name, so
-- every ascension.exe run opened TWO sessions and the card game climbed to the
-- top of the guild's weekly leaderboards. Remove it from the deckbuilding game
-- only; it keeps its own 'ascensiongame.exe', and WoW Ascension keeps
-- 'ascension.exe'. Targeted by slug (UNIQUE) rather than by name. discord.go now
-- also excludes it at seed time (wrongExecutables) so a re-seed won't re-add it.
-- Same pattern as 0021 (Enlisted falsely detected as CRSED: F.O.A.D.).

BEGIN;

UPDATE games
SET executable_names = array_remove(executable_names, 'ascension.exe')
WHERE slug = 'ascension-deckbuilding-game' AND 'ascension.exe' = ANY(executable_names);

COMMIT;
