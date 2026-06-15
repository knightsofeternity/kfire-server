-- 0023: Read-only API keys for the public /api/public/v1 surface. Keys are
-- stored as SHA-256 hashes (the cleartext is shown to the admin once). A row is
-- kept after revocation (revoked_at set) for audit.

BEGIN;

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    label        text NOT NULL,
    key_prefix   text NOT NULL,
    key_hash     bytea NOT NULL,
    created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);

CREATE UNIQUE INDEX api_keys_key_hash_idx ON api_keys (key_hash);

COMMIT;
