package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Branding is the instance's admin-set look: a named accent theme and whether
// a custom logo is uploaded.
type Branding struct {
	Accent  string
	HasLogo bool
}

// GetBranding returns the instance org's accent and logo presence. Before the
// org exists (a brand-new instance, pre first sign-up) it returns defaults so
// the public config endpoint always works.
func (s *Store) GetBranding(ctx context.Context) (Branding, error) {
	var b Branding
	err := s.pool.QueryRow(ctx,
		`SELECT accent, logo_data IS NOT NULL FROM orgs ORDER BY created_at LIMIT 1`).
		Scan(&b.Accent, &b.HasLogo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Branding{Accent: "orange"}, nil
	}
	return b, err
}

// SetAccent updates the instance org's accent theme. The caller validates the
// value against the allowed set.
func (s *Store) SetAccent(ctx context.Context, accent string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE orgs SET accent = $1
		 WHERE id = (SELECT id FROM orgs ORDER BY created_at LIMIT 1)`, accent)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetOrgLogo stores the (already validated and re-encoded) logo bytes.
func (s *Store) SetOrgLogo(ctx context.Context, contentType string, data []byte) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE orgs SET logo_data = $1, logo_content_type = $2
		 WHERE id = (SELECT id FROM orgs ORDER BY created_at LIMIT 1)`, data, contentType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOrgLogo returns the stored logo, or ErrNotFound when none is set.
func (s *Store) GetOrgLogo(ctx context.Context) (CachedImage, error) {
	var (
		ct   *string
		data []byte
	)
	err := s.pool.QueryRow(ctx,
		`SELECT logo_content_type, logo_data FROM orgs ORDER BY created_at LIMIT 1`).
		Scan(&ct, &data)
	if errors.Is(err, pgx.ErrNoRows) || data == nil {
		return CachedImage{}, ErrNotFound
	}
	if err != nil {
		return CachedImage{}, err
	}
	img := CachedImage{ContentType: "image/png", Data: data}
	if ct != nil {
		img.ContentType = *ct
	}
	return img, nil
}

// DeleteOrgLogo clears the uploaded logo.
func (s *Store) DeleteOrgLogo(ctx context.Context) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE orgs SET logo_data = NULL, logo_content_type = NULL
		 WHERE id = (SELECT id FROM orgs ORDER BY created_at LIMIT 1)`)
	return err
}
