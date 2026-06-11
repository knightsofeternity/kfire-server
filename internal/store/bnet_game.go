package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GameProfileRow is one member's cached profile blob for a game.
type GameProfileRow struct {
	UserID       string
	Username     string
	Data         []byte
	LastSyncedAt time.Time
}

// UpsertGameProfile stores a member's profile blob for one game.
func (s *Store) UpsertGameProfile(ctx context.Context, userID, gameID string, data []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bnet_game_profile (user_id, game_id, data, last_synced_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, game_id) DO UPDATE SET
			data = EXCLUDED.data, last_synced_at = now()`,
		userID, gameID, data)
	return err
}

// GameProfilesByGame returns every member's profile blob for a game, joined to
// the username, plus the newest sync time.
func (s *Store) GameProfilesByGame(ctx context.Context, gameID string) ([]GameProfileRow, time.Time, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.user_id, u.username, p.data, p.last_synced_at
		FROM bnet_game_profile p
		JOIN users u ON u.id = p.user_id AND u.banned_at IS NULL
		WHERE p.game_id = $1
		ORDER BY u.username ASC`, gameID)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var out []GameProfileRow
	var newest time.Time
	for rows.Next() {
		var r GameProfileRow
		if err := rows.Scan(&r.UserID, &r.Username, &r.Data, &r.LastSyncedAt); err != nil {
			return nil, time.Time{}, err
		}
		out = append(out, r)
		if r.LastSyncedAt.After(newest) {
			newest = r.LastSyncedAt
		}
	}
	return out, newest, rows.Err()
}

// BnetSyncedAt / MarkBnetSynced are the generic per-user/per-game refresh
// markers (table bnet_wow_sync, named historically). Used to throttle on-view
// refreshes for any Battle.net game.
func (s *Store) BnetSyncedAt(ctx context.Context, userID, gameID string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT synced_at FROM bnet_wow_sync WHERE user_id = $1 AND game_id = $2`,
		userID, gameID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

func (s *Store) MarkBnetSynced(ctx context.Context, userID, gameID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO bnet_wow_sync (user_id, game_id, synced_at) VALUES ($1, $2, now())
		 ON CONFLICT (user_id, game_id) DO UPDATE SET synced_at = now()`,
		userID, gameID)
	return err
}
