-- 0040: the Rocket League scoreboard.
--
-- A member's client now also sends the other players of the match, as numbers
-- only, and an identifier shared by every member of the same match. The rule of
-- 0035 still holds and is still STRUCTURAL: no column below can hold a name.
--
-- match_key is the SHA-256 of the game's MatchGuid, in lowercase hex. The CHECK
-- makes the column incapable of holding anything else, so it cannot become the
-- free-text column 0035 refused. Two members of the same match compute the same
-- key, which is how the recap groups their reports.
--
-- A report carries match_key and its players together or neither: the recorder
-- refuses one without the other, so a row with a key always has its players.

BEGIN;

ALTER TABLE rocket_league_matches
    ADD COLUMN match_key char(64) CHECK (match_key ~ '^[0-9a-f]{64}$');

CREATE INDEX rocket_league_matches_key_idx
    ON rocket_league_matches (match_key) WHERE match_key IS NOT NULL;

-- One row per OTHER player of a reported match. The reporting member's own line
-- stays in rocket_league_matches. position keeps the client's order, which is
-- already by team then score, never the order players arrived in.
CREATE TABLE rocket_league_match_players (
    match_id   uuid    NOT NULL REFERENCES rocket_league_matches(id) ON DELETE CASCADE,
    position   int     NOT NULL CHECK (position BETWEEN 0 AND 6),
    team       int     NOT NULL CHECK (team IN (0, 1)),
    score      int     NOT NULL CHECK (score >= 0),
    goals      int     NOT NULL CHECK (goals >= 0),
    assists    int     NOT NULL CHECK (assists >= 0),
    saves      int     NOT NULL CHECK (saves >= 0),
    shots      int     NOT NULL CHECK (shots >= 0),
    demos      int     NOT NULL CHECK (demos >= 0),
    left_early boolean NOT NULL DEFAULT false,
    PRIMARY KEY (match_id, position)
);

COMMIT;
