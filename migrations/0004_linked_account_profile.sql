-- 0004_linked_account_profile.sql - cache the external profile on link.
--
-- For Steam we display the persona name and avatar fetched from the Web API
-- at link time; profile_url deep-links to the Steam community page.

BEGIN;

ALTER TABLE linked_accounts
    ADD COLUMN avatar_url  text,
    ADD COLUMN profile_url text;

COMMIT;
