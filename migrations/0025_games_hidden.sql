-- 0025: Admin-hideable games. Hidden games are excluded from playtime stats
-- and the community games list (used to drop test-server / misdetected entries).
BEGIN;
ALTER TABLE games ADD COLUMN hidden boolean NOT NULL DEFAULT false;
COMMIT;
