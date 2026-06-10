-- 0015_sessions_visibility.sql - per-user privacy toggle for the profile's
-- recent-sessions history.
--
-- When false, other members (not self, not admins) see an empty "Recent
-- sessions" list on the user's profile. Aggregate per-game playtime and game
-- leaderboards are unaffected.

BEGIN;

ALTER TABLE users
    ADD COLUMN sessions_visible boolean NOT NULL DEFAULT true;

COMMIT;
