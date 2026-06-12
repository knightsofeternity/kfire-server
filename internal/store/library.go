package store

import "context"

// OwnedGame is a game a member is known to own, with where we learned it.
type OwnedGame struct {
	Game   Game
	Source string // "steam" or "battlenet"
}

// OwnedGames returns the union of a member's owned games: their full Steam
// library (every owned game, including never-launched ones) plus games inferred
// owned from their Battle.net profiles (WoW characters, Diablo III / StarCraft II
// profiles). Deduplicated by game, alphabetical.
func (s *Store) OwnedGames(ctx context.Context, userID string) ([]OwnedGame, error) {
	rows, err := s.pool.Query(ctx, `
		WITH owned AS (
			SELECT game_id, 'steam'::text AS source FROM external_playtime WHERE user_id = $1 AND provider = 'steam'
			UNION
			SELECT game_id, 'battlenet'   FROM bnet_wow_characters WHERE user_id = $1
			UNION
			SELECT game_id, 'battlenet'   FROM bnet_game_profile   WHERE user_id = $1
		)
		SELECT g.id, g.name, g.slug, g.icon_url, g.cover_url, min(o.source) AS source
		FROM owned o JOIN games g ON g.id = o.game_id
		GROUP BY g.id, g.name, g.slug, g.icon_url, g.cover_url
		ORDER BY g.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OwnedGame
	for rows.Next() {
		var og OwnedGame
		if err := rows.Scan(&og.Game.ID, &og.Game.Name, &og.Game.Slug, &og.Game.IconURL, &og.Game.CoverURL, &og.Source); err != nil {
			return nil, err
		}
		out = append(out, og)
	}
	return out, rows.Err()
}
