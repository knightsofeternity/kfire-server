-- 0036: playlist becomes optional.
--
-- 0035 declared it NOT NULL, following ke-rl-tracker's code, which read
-- Data.Game.Playlist. That field DOES NOT EXIST in Rocket League's real
-- protocol: verified on 2026-09-17 against the Stats API reference. A beta
-- tester played real matches and none of them was ever recorded.
--
-- The CHECK (playlist >= 0) constraint stays as is and needs no change: a
-- NULL value satisfies it, since a CHECK only rejects what evaluates to
-- false. If Psyonix ever adds this field, the column is ready to receive it.
--
-- team_size, on the other hand, IS sent by the game and stays required:
-- that is what says 1v1, 2v2 or 3v3.

BEGIN;

ALTER TABLE rocket_league_matches ALTER COLUMN playlist DROP NOT NULL;

COMMIT;
