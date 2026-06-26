-- 0027: Per-game plugin activation. Governs crawl + game-specific display.
-- Existing plugins are seeded enabled so live instances keep WoW/D3/SC2 after
-- this deploy with no admin action.
BEGIN;
CREATE TABLE game_plugins (
    id          text PRIMARY KEY,
    enabled     boolean NOT NULL DEFAULT true,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
INSERT INTO game_plugins (id, enabled) VALUES
    ('wow', true), ('d3', true), ('sc2', true)
ON CONFLICT (id) DO NOTHING;
COMMIT;
