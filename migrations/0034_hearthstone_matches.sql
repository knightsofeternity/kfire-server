-- 0034: Hearthstone match results, reported by the desktop client.
--
-- Hearthstone has no player API: Blizzard's only serves cards and deck codes.
-- The desktop client reads the game's own log files and sends a summary, so
-- this table is fed by the WebSocket control plane, not by a crawler.
--
-- What the client deliberately never sends, and what this table therefore
-- cannot hold: the opponent's name and the cards played. Both appear in the
-- log, verified on a real 120 MB Power.log on 2026-09-14, and both stay on the
-- member's machine.
--
-- placement is Battlegrounds only, 1 to 8. turns can be absent. Both are
-- nullable because a match is worth recording even when a secondary field is
-- unreadable.

BEGIN;

CREATE TABLE hearthstone_matches (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id    uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    mode       text        NOT NULL CHECK (mode IN ('battlegrounds', 'constructed')),
    result     text        NOT NULL CHECK (result IN ('win', 'loss', 'draw')),
    turns      int         CHECK (turns IS NULL OR turns >= 0),
    placement  int         CHECK (placement IS NULL OR placement BETWEEN 1 AND 8),
    played_at  timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- The client may resend a queued match after a reconnection; the same
    -- member cannot have two matches ending at the same instant.
    UNIQUE (user_id, played_at)
);

CREATE INDEX hearthstone_matches_game_idx ON hearthstone_matches (game_id, played_at DESC);

COMMIT;
