package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Session is a game session joined with its game.
type Session struct {
	ID              string
	UserID          string
	Game            Game
	Source          string
	StartedAt       time.Time
	EndedAt         *time.Time
	DurationSeconds *int
}

// sessionMergeWindow coalesces a start that arrives shortly after the same
// game's session was closed back into that session, instead of opening a new
// row. A misbehaving client that rapidly flaps a game on and off (e.g. an
// anti-cheat-protected process whose CPU reads as idle) would otherwise litter
// the history with dozens of sub-minute sessions; merging records it as one
// continuous session with the correct total duration.
const sessionMergeWindow = 90 * time.Second

// StartSession opens a session for a user and game, returning whether presence
// changed (a session was opened or reopened). It does nothing when a session is
// already open (reconnect dedup via the partial unique index). If the same
// user+game+source was closed within sessionMergeWindow, that session is
// reopened rather than a new one created, so brief detection flaps collapse
// into a single continuous session.
func (s *Store) StartSession(ctx context.Context, userID, gameID, source string) (bool, error) {
	var changed bool
	err := s.pool.QueryRow(ctx, `
		WITH reopened AS (
			UPDATE game_sessions SET ended_at = NULL
			WHERE id = (
				SELECT id FROM game_sessions
				WHERE user_id = $1 AND game_id = $2 AND source = $3
				  AND ended_at IS NOT NULL
				  AND ended_at > now() - make_interval(secs => $4)
				ORDER BY ended_at DESC
				LIMIT 1
			)
			AND NOT EXISTS (
				SELECT 1 FROM game_sessions o
				WHERE o.user_id = $1 AND o.game_id = $2 AND o.ended_at IS NULL
			)
			RETURNING id
		),
		inserted AS (
			INSERT INTO game_sessions (user_id, game_id, source)
			SELECT $1, $2, $3
			WHERE NOT EXISTS (SELECT 1 FROM reopened)
			ON CONFLICT (user_id, game_id) WHERE ended_at IS NULL DO NOTHING
			RETURNING id
		)
		SELECT EXISTS (SELECT 1 FROM reopened) OR EXISTS (SELECT 1 FROM inserted)`,
		userID, gameID, source, sessionMergeWindow.Seconds()).Scan(&changed)
	if err != nil {
		return false, err
	}
	return changed, nil
}

// EndSession closes the open session of a user+game pair. Returns whether a
// session was actually closed.
func (s *Store) EndSession(ctx context.Context, userID, gameID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE game_sessions SET ended_at = now()
		WHERE user_id = $1 AND game_id = $2 AND ended_at IS NULL`,
		userID, gameID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// EndClientSessions closes every open client-sourced session of a user (used
// when their last connection drops). Returns the number of sessions closed.
func (s *Store) EndClientSessions(ctx context.Context, userID string) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE game_sessions SET ended_at = now()
		WHERE user_id = $1 AND source = 'client' AND ended_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// LatestOpenSession returns the most recently started open session of a user,
// or nil when they are not in game.
func (s *Store) LatestOpenSession(ctx context.Context, userID string) (*Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.source, s.started_at,
		       g.id, g.name, g.slug, g.executable_names, g.platform, g.icon_url
		FROM game_sessions s
		JOIN games g ON g.id = s.game_id
		WHERE s.user_id = $1 AND s.ended_at IS NULL
		ORDER BY s.started_at DESC
		LIMIT 1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}
	var sess Session
	if err := rows.Scan(&sess.ID, &sess.UserID, &sess.Source, &sess.StartedAt,
		&sess.Game.ID, &sess.Game.Name, &sess.Game.Slug, &sess.Game.ExecutableNames,
		&sess.Game.Platform, &sess.Game.IconURL); err != nil {
		return nil, err
	}
	return &sess, nil
}

// PresenceRow is one user with their current open session, if any.
type PresenceRow struct {
	UserID          string
	Username        string
	AvatarURL       *string
	ActivityVisible bool
	PresenceStatus  string
	Game            *Game
	StartedAt       *time.Time
}

// ListPresence returns every user with their most recent open session.
func (s *Store) ListPresence(ctx context.Context) ([]PresenceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (u.id)
		       u.id, u.username, u.avatar_url, u.activity_visible, u.presence_status,
		       g.id, g.name, g.slug, g.executable_names, g.platform, g.icon_url,
		       s.started_at
		FROM users u
		LEFT JOIN game_sessions s ON s.user_id = u.id AND s.ended_at IS NULL
		LEFT JOIN games g ON g.id = s.game_id
		WHERE u.banned_at IS NULL
		ORDER BY u.id, s.started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PresenceRow
	for rows.Next() {
		var (
			r         PresenceRow
			gameID    *string
			gameName  *string
			gameSlug  *string
			exeNames  []string
			platform  *string
			iconURL   *string
			startedAt *time.Time
		)
		if err := rows.Scan(&r.UserID, &r.Username, &r.AvatarURL, &r.ActivityVisible, &r.PresenceStatus,
			&gameID, &gameName, &gameSlug, &exeNames, &platform, &iconURL,
			&startedAt); err != nil {
			return nil, err
		}
		if gameID != nil {
			r.Game = &Game{ID: *gameID, Name: *gameName, Slug: *gameSlug,
				ExecutableNames: exeNames, Platform: *platform, IconURL: iconURL}
			r.StartedAt = startedAt
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GameStat aggregates a user's playtime for one game.
type GameStat struct {
	Game         Game
	TotalSeconds int64
	SessionCount int
	LastPlayedAt time.Time
}

// UserGameStats returns a user's playtime per game, most played first. Per game
// the total is the imported platform baseline plus local sessions recorded
// since the last sync (see playtime.go); a game with no imported baseline counts
// all its local sessions. A game still appears if it was only ever played
// outside the desktop client (Steam-only) or only seen locally (non-Steam).
func (s *Store) UserGameStats(ctx context.Context, userID string) ([]GameStat, error) {
	rows, err := s.pool.Query(ctx, `
		WITH sess AS (
			SELECT game_id,
			       COALESCE(sum(duration_seconds), 0)::bigint AS secs,
			       count(*) AS cnt,
			       max(started_at) AS last_played
			FROM game_sessions WHERE user_id = $1 GROUP BY game_id
		),
		ext AS (
			SELECT game_id, sum(total_seconds)::bigint AS base, max(last_synced_at) AS synced
			FROM external_playtime WHERE user_id = $1 GROUP BY game_id
		),
		sess_since AS (
			SELECT s.game_id, COALESCE(sum(s.duration_seconds), 0)::bigint AS secs
			FROM game_sessions s JOIN ext ON ext.game_id = s.game_id
			WHERE s.user_id = $1 AND s.started_at > ext.synced
			GROUP BY s.game_id
		),
		stats AS (
			SELECT g.id, g.name, g.slug, g.icon_url,
			       (CASE WHEN ext.game_id IS NOT NULL
			             THEN ext.base + COALESCE(sess_since.secs, 0)
			             ELSE COALESCE(sess.secs, 0) END)::bigint AS total,
			       COALESCE(sess.cnt, 0) AS cnt,
			       COALESCE(sess.last_played, ext.synced) AS last_at
			FROM games g
			JOIN (
				SELECT game_id FROM sess
				UNION
				SELECT game_id FROM ext
			) played ON played.game_id = g.id
			LEFT JOIN sess ON sess.game_id = g.id
			LEFT JOIN ext  ON ext.game_id = g.id
			LEFT JOIN sess_since ON sess_since.game_id = g.id
			WHERE NOT g.hidden
		)
		-- Exclude games with no actual playtime (e.g. owned-but-unplayed Steam
		-- games the library import brings in at zero), matching ListPlayedGames.
		SELECT id, name, slug, icon_url, total, cnt, last_at
		FROM stats WHERE total > 0
		ORDER BY total DESC, cnt DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GameStat
	for rows.Next() {
		var st GameStat
		if err := rows.Scan(&st.Game.ID, &st.Game.Name, &st.Game.Slug, &st.Game.IconURL,
			&st.TotalSeconds, &st.SessionCount, &st.LastPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// SessionFilter narrows and paginates ListSessions.
type SessionFilter struct {
	UserID string
	GameID string
	From   time.Time
	To     time.Time
	Limit  int
	Cursor string
	// HideOpen excludes in-progress (live) sessions, used to keep a member's
	// current game private from other viewers.
	HideOpen bool
}

type sessionCursor struct {
	StartedAt time.Time `json:"t"`
	ID        string    `json:"id"`
}

func encodeCursor(c sessionCursor) string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (sessionCursor, error) {
	var c sessionCursor
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(raw, &c)
}

// ListSessions returns a page of session history, most recent first, plus the
// cursor of the next page ("" on the last page).
func (s *Store) ListSessions(ctx context.Context, f SessionFilter) ([]Session, string, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}

	where := []string{"true"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.UserID != "" {
		where = append(where, "s.user_id = "+arg(f.UserID))
	}
	if f.GameID != "" {
		where = append(where, "s.game_id = "+arg(f.GameID))
	}
	if f.HideOpen {
		where = append(where, "s.ended_at IS NOT NULL")
	}
	if !f.From.IsZero() {
		where = append(where, "s.started_at >= "+arg(f.From))
	}
	if !f.To.IsZero() {
		where = append(where, "s.started_at < "+arg(f.To))
	}
	if f.Cursor != "" {
		c, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		where = append(where, fmt.Sprintf("(s.started_at, s.id) < (%s, %s)",
			arg(c.StartedAt), arg(c.ID)))
	}

	// Fetch one extra row to know whether a next page exists.
	query := fmt.Sprintf(`
		SELECT s.id, s.user_id, s.source, s.started_at, s.ended_at, s.duration_seconds,
		       g.id, g.name, g.slug, g.executable_names, g.platform, g.icon_url
		FROM game_sessions s
		JOIN games g ON g.id = s.game_id
		WHERE %s
		ORDER BY s.started_at DESC, s.id DESC
		LIMIT %d`, strings.Join(where, " AND "), f.Limit+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.Source,
			&sess.StartedAt, &sess.EndedAt, &sess.DurationSeconds,
			&sess.Game.ID, &sess.Game.Name, &sess.Game.Slug,
			&sess.Game.ExecutableNames, &sess.Game.Platform, &sess.Game.IconURL); err != nil {
			return nil, "", err
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(sessions) > f.Limit {
		sessions = sessions[:f.Limit]
		last := sessions[len(sessions)-1]
		next = encodeCursor(sessionCursor{StartedAt: last.StartedAt, ID: last.ID})
	}
	return sessions, next, nil
}
