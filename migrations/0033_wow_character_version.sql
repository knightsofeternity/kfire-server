-- 0033: record which game version each WoW character came from.
--
-- The catalog slug "world-of-warcraft-classic" covers several incompatible
-- versions at once: Classic Era, Hardcore and the Classic progression realms,
-- currently Mists of Pandaria. Measured on production, that mix makes the
-- classic item level ceiling (496) HIGHER than retail's (462), so comparing
-- characters across the slug is meaningless.
--
-- The syncer already knows the version: it walks one Blizzard namespace at a
-- time. It simply threw the information away.
--
-- NULLABLE on purpose. Existing rows have no version and it cannot be guessed:
-- a level 60 character may come from Classic Era, from Hardcore, or from an
-- abandoned retail account. No backfill is needed either, because a sync
-- replaces a member's whole character set for a game, so rows heal themselves
-- on the next refresh.

BEGIN;

ALTER TABLE bnet_wow_characters ADD COLUMN version text;

COMMIT;
