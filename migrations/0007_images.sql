-- 0007_images.sql - game cover art + a lazy image cache.
--
-- Game icons/covers come from Discord's CDN. To avoid depending on it at
-- render time (and to control storage), we cache each image in Postgres the
-- first time it is actually requested - only games that are really displayed
-- ever get cached, so storage scales with usage, not the 10k+ catalog.

BEGIN;

ALTER TABLE games ADD COLUMN cover_url text;  -- Discord cover art (app-icons/{id}/{cover_hash}.png)

CREATE TABLE image_cache (
    key          text        PRIMARY KEY,   -- e.g. "<game_id>:icon" / "<game_id>:cover"
    content_type text        NOT NULL,
    data         bytea       NOT NULL,
    fetched_at   timestamptz NOT NULL DEFAULT now()
);

COMMIT;
