-- 0031: remember when the games catalog was last imported from Discord.
--
-- Until now the catalog was seeded once, on first boot, and never refreshed:
-- games added or corrected upstream never reached existing instances. The
-- server now refreshes it in the background when this timestamp is older than a
-- week (and an admin can force it from the SPA). Persisting the timestamp, as
-- opposed to keeping a ticker in memory, keeps the schedule correct across
-- restarts and redeploys.
--
-- NULL means "never synced from this column's point of view", which makes the
-- first run happen shortly after this migration is applied.

BEGIN;

ALTER TABLE orgs ADD COLUMN IF NOT EXISTS games_synced_at timestamptz;

COMMIT;
