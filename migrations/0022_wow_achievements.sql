-- 0022: per-character WoW achievements (completed list) cached as JSONB.
BEGIN;
ALTER TABLE bnet_wow_characters ADD COLUMN achievements jsonb;
COMMIT;
