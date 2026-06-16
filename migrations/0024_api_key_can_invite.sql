-- 0024: Per-key "invite" permission. Read-only keys (the v1 default) cannot
-- create invites; a key with can_invite = true may call POST
-- /api/public/v1/invites. Existing keys default to false, preserving the v1
-- read-only contract.

BEGIN;

ALTER TABLE api_keys ADD COLUMN can_invite boolean NOT NULL DEFAULT false;

COMMIT;
