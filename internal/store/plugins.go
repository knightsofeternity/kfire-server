package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// PluginStates returns the enabled flag for every plugin row, keyed by id.
func (s *Store) PluginStates(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, enabled FROM game_plugins`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		var enabled bool
		if err := rows.Scan(&id, &enabled); err != nil {
			return nil, err
		}
		out[id] = enabled
	}
	return out, rows.Err()
}

// SetPluginEnabled upserts a plugin's enabled flag.
func (s *Store) SetPluginEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO game_plugins (id, enabled, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now()
	`, id, enabled)
	return err
}

// EnsurePluginDefaults inserts a default enabled=true row for any registered
// plugin id that is not yet present. Called at boot so a newly-registered
// plugin (e.g. LoL) defaults to on.
func (s *Store) EnsurePluginDefaults(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	b := &pgx.Batch{}
	for _, id := range ids {
		b.Queue(`INSERT INTO game_plugins (id, enabled) VALUES ($1, true) ON CONFLICT (id) DO NOTHING`, id)
	}
	return s.pool.SendBatch(ctx, b).Close()
}
