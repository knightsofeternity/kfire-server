-- 0035: Rocket League match results, reported by the desktop client.
--
-- Rocket League has no public player API: the only source is a TCP socket
-- the game opens on the player's machine. The desktop client reads it,
-- aggregates it in memory and sends ONE summary per match, so this table is
-- fed by the WebSocket control plane, not by a crawler.
--
-- The privacy guarantee here is STRUCTURAL, not a rule someone has to
-- remember to enforce: the table has NO free-text column. Every column is an
-- integer, a boolean or a CHECK-bound enum, so it is incapable of holding a
-- teammate's or an opponent's pseudonym, whatever a client sends. The game's
-- own stream carries all of them; they stay on the member's machine, which
-- emits only facts about itself.
--
-- There is deliberately no arena column: a map identifier would be harmless,
-- but a free-text column would reopen the door this table closes, for
-- marginal convenience.
--
-- playlist is Psyonix's raw numeric identifier, never a label: the
-- identifier is the fact, the label is presentation and it is localized.
-- Same reasoning as hero_card_id in 0034.
--
-- duration_seconds is the actual duration measured by the client between the
-- match's opening and closing. The 7200 ceiling rejects a runaway clock
-- without forbidding a long overtime.

BEGIN;

CREATE TABLE rocket_league_matches (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id           uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    playlist          int         NOT NULL CHECK (playlist >= 0),
    team_size         int         NOT NULL CHECK (team_size BETWEEN 1 AND 4),
    player_team       int         NOT NULL CHECK (player_team IN (0, 1)),
    team_blue_score   int         NOT NULL CHECK (team_blue_score >= 0),
    team_orange_score int         NOT NULL CHECK (team_orange_score >= 0),
    result            text        NOT NULL CHECK (result IN ('win', 'loss', 'draw')),
    goals             int         NOT NULL CHECK (goals >= 0),
    assists           int         NOT NULL CHECK (assists >= 0),
    saves             int         NOT NULL CHECK (saves >= 0),
    shots             int         NOT NULL CHECK (shots >= 0),
    score             int         NOT NULL CHECK (score >= 0),
    demos             int         NOT NULL CHECK (demos >= 0),
    mvp               boolean     NOT NULL DEFAULT false,
    duration_seconds  int         NOT NULL CHECK (duration_seconds BETWEEN 0 AND 7200),
    played_at         timestamptz NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    -- The client may resend a queued match after a reconnection; the same
    -- member cannot have two matches ending at the same instant.
    UNIQUE (user_id, played_at)
);

CREATE INDEX rocket_league_matches_game_idx ON rocket_league_matches (game_id, played_at DESC);

COMMIT;
