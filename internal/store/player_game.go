package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// WowCharactersForUserGame returns one member's WoW characters for a game.
func (s *Store) WowCharactersForUserGame(ctx context.Context, userID, gameID string) ([]WowCharacterRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT user_id, region, realm_slug, realm_name, name, faction, race, class,
		       level, item_level, mythic_rating, raid_summary, achievement_points, last_synced_at
		FROM bnet_wow_characters WHERE user_id = $1 AND game_id = $2
		ORDER BY item_level DESC, name ASC`, userID, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WowCharacterRow
	for rows.Next() {
		var r WowCharacterRow
		r.GameID = gameID
		if err := rows.Scan(&r.UserID, &r.Region, &r.RealmSlug, &r.RealmName, &r.Name, &r.Faction,
			&r.Race, &r.Class, &r.Level, &r.ItemLevel, &r.MythicRating, &r.RaidSummary, &r.AchievementPoints, &r.LastSyncedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// WowCharacterAchievements returns one character's cached achievements JSONB
// (the raw [] of {id,name,completed_at}), or nil if absent.
func (s *Store) WowCharacterAchievements(ctx context.Context, userID, realmSlug, name string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `
		SELECT achievements FROM bnet_wow_characters
		WHERE user_id = $1 AND realm_slug = $2 AND lower(name) = lower($3)`,
		userID, realmSlug, name).Scan(&data)
	if err != nil {
		return nil, nil // not found / no data -> empty
	}
	return data, nil
}

// GameProfileForUserGame returns one member's bnet profile blob for a game (nil if none).
func (s *Store) GameProfileForUserGame(ctx context.Context, userID, gameID string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM bnet_game_profile WHERE user_id = $1 AND game_id = $2`, userID, gameID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
