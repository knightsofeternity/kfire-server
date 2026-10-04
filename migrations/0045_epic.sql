-- Epic Games connector: a member links their own Epic account (no bot).
-- The sealed refresh token lives in linked_accounts.refresh_token_enc
-- (provider 'epic' is allowed since 0001); this table keeps its expiry and the
-- link's state.
CREATE TABLE epic_accounts (
    user_id            uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    refresh_expires_at timestamptz,
    status             text NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'needs_relink')),
    last_synced_at     timestamptz,
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- Catalog answers, kept so an item is resolved once for every member.
-- is_game: category "games" present and "hidden" absent (add-on content such
-- as "Contenu de LEGO Fortnite" is hidden and must not count playtime).
CREATE TABLE epic_titles (
    namespace       text NOT NULL,
    catalog_item_id text NOT NULL,
    app_name        text NOT NULL,
    title           text NOT NULL,
    is_game         boolean NOT NULL,
    image_url       text,
    game_id         uuid REFERENCES games(id) ON DELETE SET NULL,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, catalog_item_id)
);

ALTER TABLE external_playtime DROP CONSTRAINT IF EXISTS external_playtime_provider_check;
ALTER TABLE external_playtime ADD CONSTRAINT external_playtime_provider_check
    CHECK (provider IN ('steam', 'xbox', 'psn', 'nintendo', 'wow_addon', 'epic'));
