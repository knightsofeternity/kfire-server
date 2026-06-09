-- 0002_games_discord.sql - track the Discord "detectable applications" origin
-- of seeded games so re-syncs can upsert on a stable key.

BEGIN;

ALTER TABLE games
    ADD COLUMN discord_app_id text UNIQUE;  -- NULL for admin-added custom games

COMMIT;
