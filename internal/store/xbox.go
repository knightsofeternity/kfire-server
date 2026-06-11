package store

import "context"

// GameByXboxTitleID resolves an Xbox title id to a catalog game, or ErrNotFound.
func (s *Store) GameByXboxTitleID(ctx context.Context, titleID string) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, icon_url FROM games WHERE xbox_title_id = $1 LIMIT 1`, titleID).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err != nil {
		return g, ErrNotFound
	}
	return g, nil
}

// UpsertXboxGame resolves an Xbox title to a catalog game: by xbox_title_id, then
// by slug (adopting the title id), else creates an entry with no executables.
func (s *Store) UpsertXboxGame(ctx context.Context, titleID, name string) (Game, error) {
	if g, err := s.GameByXboxTitleID(ctx, titleID); err == nil {
		return g, nil
	}
	slug := steamSlug(name)
	var g Game
	err := s.pool.QueryRow(ctx, `
		UPDATE games SET xbox_title_id = $1 WHERE slug = $2
		RETURNING id, name, slug, icon_url`, titleID, slug).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	if err == nil {
		return g, nil
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO games (name, slug, executable_names, platform, xbox_title_id)
		VALUES ($1, $2, '{}', 'xbox', $3)
		RETURNING id, name, slug, icon_url`, name, slug, titleID).
		Scan(&g.ID, &g.Name, &g.Slug, &g.IconURL)
	return g, err
}

// OpenSessionBySource returns a user's open session for a given source, or nil.
func (s *Store) OpenSessionBySource(ctx context.Context, userID, source string) (*Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.source, s.started_at,
		       g.id, g.name, g.slug, g.executable_names, g.platform, g.icon_url
		FROM game_sessions s JOIN games g ON g.id = s.game_id
		WHERE s.user_id = $1 AND s.source = $2 AND s.ended_at IS NULL
		ORDER BY s.started_at DESC LIMIT 1`, userID, source)
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

// LinkedTokenUser is a linked account with its encrypted token (poller input).
type LinkedTokenUser struct {
	UserID         string
	ProviderUserID string
	AccessTokenEnc []byte
}

// ListLinkedTokensByProvider returns every linked account of a provider with its
// encrypted access token (the poller decrypts and uses it).
func (s *Store) ListLinkedTokensByProvider(ctx context.Context, provider string) ([]LinkedTokenUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, provider_user_id, access_token_enc FROM linked_accounts WHERE provider = $1`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkedTokenUser
	for rows.Next() {
		var u LinkedTokenUser
		if err := rows.Scan(&u.UserID, &u.ProviderUserID, &u.AccessTokenEnc); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
