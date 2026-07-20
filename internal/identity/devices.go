package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrDeviceRevoked = errors.New("device has been revoked")

type RegisteredDevice struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	UserID      uuid.UUID  `json:"user_id"`
	DeviceName  *string    `json:"device_name,omitempty"`
	Platform    *string    `json:"platform,omitempty"`
	Fingerprint *string    `json:"fingerprint,omitempty"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

type RegisterDeviceInput struct {
	ID          *uuid.UUID
	DeviceName  *string
	Platform    *string
	Fingerprint *string
}

func (s *Service) RegisterDevice(ctx context.Context, tenantID, userID uuid.UUID, in RegisterDeviceInput) (*RegisteredDevice, error) {
	id := uuid.New()
	if in.ID != nil && *in.ID != uuid.Nil {
		id = *in.ID
	}
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity.registered_devices (id, tenant_id, user_id, device_name, platform, fingerprint, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			device_name = COALESCE(EXCLUDED.device_name, identity.registered_devices.device_name),
			platform = COALESCE(EXCLUDED.platform, identity.registered_devices.platform),
			fingerprint = COALESCE(EXCLUDED.fingerprint, identity.registered_devices.fingerprint),
			last_seen_at = EXCLUDED.last_seen_at
		WHERE identity.registered_devices.revoked_at IS NULL`,
		id, tenantID, userID, in.DeviceName, in.Platform, in.Fingerprint, now)
	if err != nil {
		return nil, err
	}
	return s.getDevice(ctx, tenantID, id)
}

func (s *Service) ListDevices(ctx context.Context, tenantID, userID uuid.UUID) ([]RegisteredDevice, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, device_name, platform, fingerprint, last_seen_at, created_at, revoked_at
		FROM identity.registered_devices
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RegisteredDevice, 0)
	for rows.Next() {
		var d RegisteredDevice
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.UserID, &d.DeviceName, &d.Platform, &d.Fingerprint,
			&d.LastSeenAt, &d.CreatedAt, &d.RevokedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (s *Service) RevokeDevice(ctx context.Context, tenantID, userID, deviceID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE identity.registered_devices
		SET revoked_at = now()
		WHERE tenant_id = $1 AND user_id = $2 AND id = $3 AND revoked_at IS NULL`,
		tenantID, userID, deviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) AssertDeviceActive(ctx context.Context, tenantID, deviceID uuid.UUID) error {
	var revokedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT revoked_at FROM identity.registered_devices
		WHERE tenant_id = $1 AND id = $2`, tenantID, deviceID).Scan(&revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if revokedAt != nil {
		return ErrDeviceRevoked
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE identity.registered_devices SET last_seen_at = now()
		WHERE tenant_id = $1 AND id = $2 AND revoked_at IS NULL`, tenantID, deviceID)
	return nil
}

func (s *Service) getDevice(ctx context.Context, tenantID, deviceID uuid.UUID) (*RegisteredDevice, error) {
	var d RegisteredDevice
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, device_name, platform, fingerprint, last_seen_at, created_at, revoked_at
		FROM identity.registered_devices
		WHERE tenant_id = $1 AND id = $2`, tenantID, deviceID).Scan(
		&d.ID, &d.TenantID, &d.UserID, &d.DeviceName, &d.Platform, &d.Fingerprint,
		&d.LastSeenAt, &d.CreatedAt, &d.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: device", ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}
