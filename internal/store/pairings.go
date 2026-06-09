package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Pairing is a device-linking request.
type Pairing struct {
	DeviceCode string
	UserCode   string
	DeviceID   string
	DeviceName string
	Platform   string
	Status     string
	UserID     *string
	ExpiresAt  time.Time
}

// CreatePairing stores a new pending pairing.
func (s *Store) CreatePairing(ctx context.Context, p Pairing) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO device_pairings
			(device_code, user_code, device_id, device_name, platform, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		p.DeviceCode, p.UserCode, p.DeviceID, p.DeviceName, p.Platform, p.ExpiresAt)
	return err
}

// GetPendingPairingByUserCode returns a pending, unexpired pairing for the
// approval page to display.
func (s *Store) GetPendingPairingByUserCode(ctx context.Context, userCode string) (Pairing, error) {
	var p Pairing
	err := s.pool.QueryRow(ctx, `
		SELECT device_code, user_code, device_id, device_name, platform, status, expires_at
		FROM device_pairings
		WHERE user_code = $1 AND status = 'pending' AND expires_at > now()`, userCode).
		Scan(&p.DeviceCode, &p.UserCode, &p.DeviceID, &p.DeviceName, &p.Platform, &p.Status, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pairing{}, ErrNotFound
	}
	return p, err
}

// ApprovePairing binds a pending pairing to the approving user.
func (s *Store) ApprovePairing(ctx context.Context, userCode, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE device_pairings SET status = 'approved', user_id = $2
		WHERE user_code = $1 AND status = 'pending' AND expires_at > now()`,
		userCode, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPairingByDeviceCode returns a pairing for polling.
func (s *Store) GetPairingByDeviceCode(ctx context.Context, deviceCode string) (Pairing, error) {
	var p Pairing
	err := s.pool.QueryRow(ctx, `
		SELECT device_code, user_code, device_id, device_name, platform, status, user_id, expires_at
		FROM device_pairings WHERE device_code = $1`, deviceCode).
		Scan(&p.DeviceCode, &p.UserCode, &p.DeviceID, &p.DeviceName, &p.Platform,
			&p.Status, &p.UserID, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pairing{}, ErrNotFound
	}
	return p, err
}

// ClaimPairing marks an approved pairing as claimed (single use).
func (s *Store) ClaimPairing(ctx context.Context, deviceCode string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE device_pairings SET status = 'claimed'
		WHERE device_code = $1 AND status = 'approved'`, deviceCode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
