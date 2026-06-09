-- 0006_invites.sql - admin member onboarding via shareable invite links.

BEGIN;

CREATE TABLE invites (
    code       text PRIMARY KEY,                 -- random, embedded in the link
    note       text,                             -- admin's free-text reminder (who it's for)
    role       text        NOT NULL DEFAULT 'member'
               CHECK (role IN ('admin', 'member')),
    created_by uuid        REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,                       -- NULL while pending
    used_by    uuid        REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX invites_pending_idx ON invites (created_at DESC) WHERE used_at IS NULL;

COMMIT;
