-- 0021: Discord lists Enlisted's binary (enlisted.exe) under CRSED: F.O.A.D.
-- (same studio), so playing Enlisted was falsely detected as CRSED. Remove it
-- from CRSED only; Enlisted keeps its own enlisted.exe. discord.go now also
-- excludes it at seed time (wrongExecutables) so a re-seed won't re-add it.

BEGIN;

UPDATE games
SET executable_names = array_remove(executable_names, 'enlisted.exe')
WHERE name = 'CRSED: F.O.A.D.' AND 'enlisted.exe' = ANY(executable_names);

COMMIT;
