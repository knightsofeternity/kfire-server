-- 0010_org_branding.sql - per-instance branding the admin can set: the dominant
-- accent color (a named theme) and an uploaded clan/team logo stored as bytes.

BEGIN;

ALTER TABLE orgs
    ADD COLUMN accent            text NOT NULL DEFAULT 'orange',
    ADD COLUMN logo_data         bytea,
    ADD COLUMN logo_content_type text;

COMMIT;
