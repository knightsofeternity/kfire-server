-- 0014_password_resets.sql - single-use, short-lived password reset tokens an
-- admin generates for a member (no email: the admin shares the link directly).
-- Only the SHA-256 hash of the token is stored.

BEGIN;

CREATE TABLE password_resets (
    token_hash bytea       PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX password_resets_user_idx ON password_resets (user_id);

COMMIT;
