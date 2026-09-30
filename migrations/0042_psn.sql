-- 0042: the PlayStation connector.
--
-- One bot account per instance reads its friends' presence, played games and
-- trophies. Its NPSSO is the only secret: pasted by an admin, stored sealed
-- with the master key like OAuth tokens, never returned by the API.
--
-- A PlayStation game has one concept id across PS4, PS5 and regions, but a
-- title id per platform and region. games.psn_concept_id ties the concept to a
-- catalog game; psn_titles maps every title id seen to that game, so presence,
-- which only names a title id, finds it.

BEGIN;

CREATE TABLE psn_bot (
    id                 int         PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    npsso_enc          bytea       NOT NULL,
    refresh_token_enc  bytea,
    refresh_expires_at timestamptz,
    access_token_enc   bytea,
    access_expires_at  timestamptz,
    online_id          text,
    account_id         text,
    status             text        NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'needs_npsso')),
    last_error         text,
    npsso_set_at       timestamptz NOT NULL DEFAULT now(),
    last_ok_at         timestamptz
);

ALTER TABLE games ADD COLUMN psn_concept_id text;
CREATE INDEX games_psn_concept_idx ON games (psn_concept_id) WHERE psn_concept_id IS NOT NULL;

CREATE TABLE psn_titles (
    np_title_id text PRIMARY KEY,
    game_id     uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE
);

-- When the member's library was last imported, to space imports by 6 hours.
CREATE TABLE psn_library_sync (
    user_id   uuid        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    synced_at timestamptz NOT NULL DEFAULT now()
);

COMMIT;
