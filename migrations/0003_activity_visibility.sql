-- 0003_activity_visibility.sql — per-user privacy toggle.
--
-- When false, other members see the user capped at "online" with no game in
-- live presence (snapshot + WebSocket). The user's own sessions are still
-- recorded and visible on their own profile.

BEGIN;

ALTER TABLE users
    ADD COLUMN activity_visible boolean NOT NULL DEFAULT true;

COMMIT;
