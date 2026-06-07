package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Game mirrors a row of the games table.
type Game struct {
	ID              string
	Name            string
	Slug            string
	ExecutableNames []string
	Platform        string
	IconURL         *string
}

// GameSeed is a normalized entry from an external catalog (Discord).
type GameSeed struct {
	DiscordAppID    string
	Name            string
	Slug            string
	ExecutableNames []string
	IconURL         string
}

// CountGames returns the catalog size.
func (s *Store) CountGames(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM games`).Scan(&n)
	return n, err
}

// ListGames returns the whole catalog (the desktop client downloads it to
// match local processes).
func (s *Store) ListGames(ctx context.Context) ([]Game, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, executable_names, platform, icon_url
		FROM games ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug, &g.ExecutableNames, &g.Platform, &g.IconURL); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// GetGameBySlug fetches one game.
func (s *Store) GetGameBySlug(ctx context.Context, slug string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, executable_names, platform, icon_url
		FROM games WHERE slug = $1`, slug).
		Scan(&g.ID, &g.Name, &g.Slug, &g.ExecutableNames, &g.Platform, &g.IconURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Game{}, ErrNotFound
	}
	return g, err
}

// UpsertGames inserts or updates seeded games keyed on discord_app_id.
// Slugs are stable: an existing game keeps its slug across re-syncs; new
// games get a unique slug (name collisions are suffixed with the app id).
func (s *Store) UpsertGames(ctx context.Context, seeds []GameSeed) (int, error) {
	// Existing slug assignments, to keep slugs stable and unique.
	rows, err := s.pool.Query(ctx, `SELECT slug, COALESCE(discord_app_id, '') FROM games`)
	if err != nil {
		return 0, err
	}
	slugByApp := make(map[string]string)
	taken := make(map[string]struct{})
	for rows.Next() {
		var slug, appID string
		if err := rows.Scan(&slug, &appID); err != nil {
			rows.Close()
			return 0, err
		}
		taken[slug] = struct{}{}
		if appID != "" {
			slugByApp[appID] = slug
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	const chunkSize = 500
	upserted := 0
	for start := 0; start < len(seeds); start += chunkSize {
		chunk := seeds[start:min(start+chunkSize, len(seeds))]

		batch := &pgx.Batch{}
		for _, seed := range chunk {
			slug, exists := slugByApp[seed.DiscordAppID]
			if !exists {
				slug = seed.Slug
				if _, dup := taken[slug]; dup {
					slug = fmt.Sprintf("%s-%s", slug, seed.DiscordAppID)
				}
				taken[slug] = struct{}{}
				slugByApp[seed.DiscordAppID] = slug
			}

			batch.Queue(`
				INSERT INTO games (name, slug, executable_names, platform, icon_url, discord_app_id)
				VALUES ($1, $2, $3, 'pc', NULLIF($4, ''), $5)
				ON CONFLICT (discord_app_id) DO UPDATE SET
					name             = EXCLUDED.name,
					executable_names = EXCLUDED.executable_names,
					icon_url         = EXCLUDED.icon_url`,
				seed.Name, slug, seed.ExecutableNames, seed.IconURL, seed.DiscordAppID)
		}

		if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
			return upserted, fmt.Errorf("upsert games batch at %d: %w", start, err)
		}
		upserted += len(chunk)
	}
	return upserted, nil
}
