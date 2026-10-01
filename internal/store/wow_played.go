package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// WowPlayed is one character's /played, as the KFire addon recorded it.
type WowPlayed struct {
	Region        string
	RealmNorm     string
	Realm         string
	Name          string
	PlayedSeconds int64
	Level         *int
	Class         *string
	RecordedAt    time.Time
}

// WowPlayedPrevious is what is already stored for one character, to judge a
// new record against.
type WowPlayedPrevious struct {
	PlayedSeconds int64
	RecordedAt    time.Time
}

// WowPlayedPrevious returns the stored record of one character, or nil.
func (s *Store) WowPlayedPrevious(ctx context.Context, userID, gameID string, c WowPlayed) (*WowPlayedPrevious, error) {
	var p WowPlayedPrevious
	err := s.pool.QueryRow(ctx, `
		SELECT played_seconds, recorded_at FROM wow_played
		WHERE user_id = $1 AND game_id = $2 AND region = $3 AND realm_norm = $4 AND name = $5`,
		userID, gameID, c.Region, c.RealmNorm, c.Name).Scan(&p.PlayedSeconds, &p.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertWowPlayed stores one character for a member and edition. A row is only
// replaced by a more recent record, so a stale file from another account
// folder never takes hours back.
func (s *Store) UpsertWowPlayed(ctx context.Context, userID, gameID string, c WowPlayed) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO wow_played (user_id, game_id, region, realm_norm, name, realm,
		                        played_seconds, level, class, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, game_id, region, realm_norm, name) DO UPDATE SET
		    realm = EXCLUDED.realm, played_seconds = EXCLUDED.played_seconds,
		    level = EXCLUDED.level, class = EXCLUDED.class,
		    recorded_at = EXCLUDED.recorded_at, updated_at = now()
		WHERE EXCLUDED.recorded_at > wow_played.recorded_at`,
		userID, gameID, c.Region, c.RealmNorm, c.Name, c.Realm,
		c.PlayedSeconds, c.Level, c.Class, c.RecordedAt)
	return err
}

// SetExternalPlaytime replaces a member's platform playtime outright, unlike
// UpsertExternalPlaytime which never lowers it. Used when a source corrects
// itself downwards and the old, higher figure is known to be wrong.
func (s *Store) SetExternalPlaytime(ctx context.Context, userID, provider, gameID string, totalSeconds int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO external_playtime (user_id, provider, game_id, total_seconds, last_synced_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id, provider, game_id) DO UPDATE SET
			total_seconds = EXCLUDED.total_seconds, last_synced_at = now()`,
		userID, provider, gameID, totalSeconds)
	return err
}

// WowPlayedTotal sums a member's /played on one edition.
func (s *Store) WowPlayedTotal(ctx context.Context, userID, gameID string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(played_seconds), 0) FROM wow_played
		WHERE user_id = $1 AND game_id = $2`, userID, gameID).Scan(&total)
	return total, err
}

// WowPlayedForUserGame lists a member's characters on one edition, most
// played first.
func (s *Store) WowPlayedForUserGame(ctx context.Context, userID, gameID string) ([]WowPlayed, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT region, realm_norm, realm, name, played_seconds, level, class, recorded_at
		FROM wow_played WHERE user_id = $1 AND game_id = $2
		ORDER BY played_seconds DESC, name`, userID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WowPlayed
	for rows.Next() {
		var c WowPlayed
		if err := rows.Scan(&c.Region, &c.RealmNorm, &c.Realm, &c.Name, &c.PlayedSeconds,
			&c.Level, &c.Class, &c.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
