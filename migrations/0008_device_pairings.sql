-- 0008_device_pairings.sql — browser-based device linking (OAuth device grant).
--
-- A desktop client links to the server without typing credentials: it starts a
-- pairing, the user approves it from the already-authenticated web app, and the
-- client polls until it receives device-bound tokens.

BEGIN;

CREATE TABLE device_pairings (
    device_code text        PRIMARY KEY,        -- secret, held by the client
    user_code   text        NOT NULL UNIQUE,    -- short, shown to the user / in the link
    device_id   uuid        NOT NULL,
    device_name text        NOT NULL,
    platform    text        NOT NULL
                CHECK (platform IN ('windows', 'macos', 'linux')),
    status      text        NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'approved', 'claimed')),
    user_id     uuid        REFERENCES users(id) ON DELETE CASCADE,  -- set on approval
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL
);

CREATE INDEX device_pairings_user_code_idx ON device_pairings (user_code) WHERE status = 'pending';

COMMIT;
