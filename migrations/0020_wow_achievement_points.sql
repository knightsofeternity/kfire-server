-- 0020: store WoW achievement points per character (from the character summary).
BEGIN;
ALTER TABLE bnet_wow_characters ADD COLUMN achievement_points int NOT NULL DEFAULT 0;
COMMIT;
