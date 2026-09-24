-- 0039: Battlegrounds rating, when the member's client could read it.
--
-- The game's own logs never carry the rating. Hearthstone Deck Tracker writes
-- it in its own file; the desktop client reads that file, as an option, for the
-- members who installed HDT. Both columns are nullable on purpose: most
-- matches will have no rating (no HDT, HDT not running, constructed game), and
-- a match is always worth recording without one.
--
-- 20000 is far above any real Battlegrounds rating; the bound exists so a
-- corrupt value cannot land in the guild's leaderboard.

BEGIN;

ALTER TABLE hearthstone_matches
    ADD COLUMN rating       int CHECK (rating       IS NULL OR rating       BETWEEN 0 AND 20000),
    ADD COLUMN rating_after int CHECK (rating_after IS NULL OR rating_after BETWEEN 0 AND 20000);

COMMIT;
