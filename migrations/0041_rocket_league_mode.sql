-- 0041: the Rocket League mode, when the arena gives it away.
--
-- The game's stats stream names no playlist, but it does name the arena, and
-- Hoops and Dropshot are played on arenas of their own. The client sends the
-- arena code; the server keeps only the mode it implies. The column is a closed
-- set, so the rule of 0035 holds: nothing here can hold free text.
--
-- NULL means a standard arena, where ranked, casual, Rumble and Heatseeker all
-- look the same, or a match reported by a client that sent no arena.

BEGIN;

ALTER TABLE rocket_league_matches
    ADD COLUMN mode text CHECK (mode IN ('hoops', 'dropshot'));

COMMIT;
