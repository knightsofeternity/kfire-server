-- 0019: map Xbox presence titles to catalog games (mirrors steam_app_id).
BEGIN;
ALTER TABLE games ADD COLUMN xbox_title_id text;
CREATE INDEX games_xbox_title_id_idx ON games (xbox_title_id) WHERE xbox_title_id IS NOT NULL;
COMMIT;
