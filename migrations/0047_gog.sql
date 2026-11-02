-- GOG connector: a member links their own GOG account (no bot), like Epic.
-- The sealed refresh token lives in linked_accounts.refresh_token_enc; this
-- table keeps the Galaxy user id every per-user URL needs and the link's state.
ALTER TABLE linked_accounts DROP CONSTRAINT IF EXISTS linked_accounts_provider_check;
ALTER TABLE linked_accounts ADD CONSTRAINT linked_accounts_provider_check
    CHECK (provider IN ('steam', 'battlenet', 'riot', 'epic', 'xbox', 'psn', 'pubg', 'nintendo', 'gog'));

CREATE TABLE gog_accounts (
    user_id        uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    galaxy_user_id text NOT NULL,
    status         text NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'needs_relink')),
    last_synced_at timestamptz,
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Public catalog answers, kept so a product is resolved once for every member.
-- is_game: game_type "game" (DLC and packs must not show as games).
CREATE TABLE gog_titles (
    product_id text PRIMARY KEY,
    title      text NOT NULL,
    is_game    boolean NOT NULL,
    image_url  text,
    game_id    uuid REFERENCES games(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE external_playtime DROP CONSTRAINT IF EXISTS external_playtime_provider_check;
ALTER TABLE external_playtime ADD CONSTRAINT external_playtime_provider_check
    CHECK (provider IN ('steam', 'xbox', 'psn', 'nintendo', 'wow_addon', 'epic', 'gog'));
