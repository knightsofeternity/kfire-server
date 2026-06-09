package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// CachedImage is a stored game image (icon or cover).
type CachedImage struct {
	ContentType string
	Data        []byte
}

// GetCachedImage returns a cached image, or ErrNotFound.
func (s *Store) GetCachedImage(ctx context.Context, key string) (CachedImage, error) {
	var img CachedImage
	err := s.pool.QueryRow(ctx,
		`SELECT content_type, data FROM image_cache WHERE key = $1`, key).
		Scan(&img.ContentType, &img.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return CachedImage{}, ErrNotFound
	}
	return img, err
}

// PutCachedImage stores (or refreshes) a cached image.
func (s *Store) PutCachedImage(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO image_cache (key, content_type, data, fetched_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (key) DO UPDATE SET
			content_type = EXCLUDED.content_type,
			data         = EXCLUDED.data,
			fetched_at   = now()`,
		key, contentType, data)
	return err
}
