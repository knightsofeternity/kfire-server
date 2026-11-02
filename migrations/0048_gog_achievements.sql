-- GOG achievements: each game keeps them under its own Galaxy client, read
-- from the game's public build manifest once for every member. A game without
-- a Galaxy build keeps client_checked_at set and its client empty, and is
-- looked at again a week later.
ALTER TABLE gog_titles
    ADD COLUMN galaxy_client_id     text,
    ADD COLUMN galaxy_client_secret text,
    ADD COLUMN client_checked_at    timestamptz;

ALTER TABLE achievements DROP CONSTRAINT IF EXISTS achievements_provider_check;
ALTER TABLE achievements ADD CONSTRAINT achievements_provider_check
    CHECK (provider IN ('steam', 'xbox', 'psn', 'gog'));
