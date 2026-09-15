package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// WowCharacterRow is one stored WoW character.
type WowCharacterRow struct {
	UserID            string
	GameID            string
	Username          string
	AvatarURL         *string
	Region            string
	RealmSlug         string
	Name              string
	RealmName         string
	Faction           *string
	Race              *string
	Class             *string
	Level             int
	ItemLevel         int
	MythicRating      *float64
	RaidSummary       []byte
	AchievementPoints int
	Achievements      []byte
	Version           *string
	// HasAchievements says whether Blizzard ever returned an achievement list
	// for this character. It is false when the character profile answers 404,
	// which Blizzard does for characters left unplayed for a while, and the
	// page must say so instead of showing an empty list.
	HasAchievements bool
	LastSyncedAt    time.Time
}

// ReplaceWowCharacters atomically replaces a member's WoW characters for one
// game (so deleted/transferred characters disappear), stamping last_synced_at.
func (s *Store) ReplaceWowCharacters(ctx context.Context, userID, gameID string, chars []WowCharacterRow) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM bnet_wow_characters WHERE user_id = $1 AND game_id = $2`, userID, gameID); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, ch := range chars {
		batch.Queue(`
			INSERT INTO bnet_wow_characters
				(user_id, game_id, region, realm_slug, realm_name, name, faction, race, class,
				 level, item_level, mythic_rating, raid_summary, achievement_points, achievements,
				 version, last_synced_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16, now())`,
			userID, gameID, ch.Region, ch.RealmSlug, ch.RealmName, ch.Name, ch.Faction, ch.Race,
			ch.Class, ch.Level, ch.ItemLevel, ch.MythicRating, ch.RaidSummary, ch.AchievementPoints,
			ch.Achievements, ch.Version)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WowCharactersByGame returns every character for a game, joined to its owner's
// name so the page can group by member, plus the newest sync time.
//
// Banned members are excluded, matching the Riot aggregate. The ordering is a
// stable default; the interface regroups by member and by version.
func (s *Store) WowCharactersByGame(ctx context.Context, gameID string) ([]WowCharacterRow, time.Time, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.user_id, u.username, u.avatar_url, c.region, c.realm_slug, c.realm_name,
		       c.name, c.faction, c.race, c.class, c.level, c.item_level, c.mythic_rating,
		       c.raid_summary, c.achievement_points, c.version, c.last_synced_at
		FROM bnet_wow_characters c
		JOIN users u ON u.id = c.user_id AND u.banned_at IS NULL
		WHERE c.game_id = $1
		ORDER BY c.level DESC, c.item_level DESC, c.name ASC`, gameID)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var out []WowCharacterRow
	var newest time.Time
	for rows.Next() {
		var r WowCharacterRow
		r.GameID = gameID
		if err := rows.Scan(&r.UserID, &r.Username, &r.AvatarURL, &r.Region, &r.RealmSlug,
			&r.RealmName, &r.Name, &r.Faction, &r.Race, &r.Class, &r.Level, &r.ItemLevel,
			&r.MythicRating, &r.RaidSummary, &r.AchievementPoints, &r.Version,
			&r.LastSyncedAt); err != nil {
			return nil, time.Time{}, err
		}
		out = append(out, r)
		if r.LastSyncedAt.After(newest) {
			newest = r.LastSyncedAt
		}
	}
	return out, newest, rows.Err()
}

// WowSyncedAt returns when this member's WoW characters for a game were last
// refreshed (zero time if never).
func (s *Store) WowSyncedAt(ctx context.Context, userID, gameID string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT synced_at FROM bnet_wow_sync WHERE user_id = $1 AND game_id = $2`,
		userID, gameID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

// MarkWowSynced records that this member's WoW characters for a game were just
// refreshed (used to throttle on-view refreshes per user).
func (s *Store) MarkWowSynced(ctx context.Context, userID, gameID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO bnet_wow_sync (user_id, game_id, synced_at) VALUES ($1, $2, now())
		 ON CONFLICT (user_id, game_id) DO UPDATE SET synced_at = now()`,
		userID, gameID)
	return err
}
