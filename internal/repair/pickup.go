package repair

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	pickupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// PickupQRPayload is the deep-link encoded in the intake-slip QR so Scan opens
// the repair collect path rather than commerce order pickup.
func PickupQRPayload(code string) string {
	return "techlane://repair-pickup/" + strings.ToUpper(strings.TrimSpace(code))
}

func generatePickupCode() (string, error) {
	out := make([]byte, 6)
	for i := range out {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(pickupCodeAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = pickupCodeAlphabet[v.Int64()]
	}
	return "PK-" + string(out), nil
}

func (s *Service) allocatePickupCode(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		code, err := generatePickupCode()
		if err != nil {
			return "", err
		}
		var conflict bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM repair.repair_jobs WHERE tenant_id = $1 AND pickup_code = $2
			)`, tenantID, code).Scan(&conflict); err != nil {
			return "", err
		}
		if !conflict {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not allocate a unique pickup code")
}

// EnsurePickupCode assigns a durable pickup code if the job never got one
// (jobs created before the feature, or a rare race). Safe to call repeatedly.
func (s *Service) EnsurePickupCode(ctx context.Context, tenantID, repairID uuid.UUID) (string, error) {
	var existing *string
	err := s.pool.QueryRow(ctx, `
		SELECT pickup_code FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`,
		tenantID, repairID).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("repair not found")
	}
	if err != nil {
		return "", err
	}
	if existing != nil && strings.TrimSpace(*existing) != "" {
		return strings.ToUpper(*existing), nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	code, err := s.allocatePickupCode(ctx, tx, tenantID)
	if err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs SET pickup_code = $1, updated_at = now()
		WHERE tenant_id = $2 AND id = $3 AND (pickup_code IS NULL OR pickup_code = '')`,
		code, tenantID, repairID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		// Concurrent writer won — re-read.
		var got string
		if err := tx.QueryRow(ctx, `
			SELECT pickup_code FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
			tenantID, repairID).Scan(&got); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return strings.ToUpper(got), nil
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return code, nil
}

// NormalizePickupCode extracts a PK-… claim code from typed input, a deep link,
// or a raw QR payload. Empty string means no pickup code was present.
func NormalizePickupCode(raw string) string {
	v := strings.ToUpper(strings.TrimSpace(raw))
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(v, "TECHLANE://REPAIR-PICKUP/")
	v = strings.TrimPrefix(v, "TECHLANE:REPAIR-PICKUP:")
	if i := strings.Index(v, "PK-"); i >= 0 {
		end := i + 3
		for end < len(v) {
			c := v[end]
			if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
				end++
				continue
			}
			break
		}
		return v[i:end]
	}
	return v
}

// FindRepairByPickupCode resolves the printed claim code to a job id.
func (s *Service) FindRepairByPickupCode(ctx context.Context, tenantID uuid.UUID, code string) (uuid.UUID, error) {
	code = NormalizePickupCode(code)
	if code == "" {
		return uuid.Nil, fmt.Errorf("pickup code required")
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM repair.repair_jobs
		WHERE tenant_id = $1 AND UPPER(pickup_code) = UPPER($2) AND deleted_at IS NULL`,
		tenantID, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("no repair matches that pickup code")
	}
	return id, err
}

// CollectByPickupCode is the counter / scan path: present the intake slip (or type
// the code), and if the job is ready and paid, mark it collected immediately.
func (s *Service) CollectByPickupCode(ctx context.Context, tenantID uuid.UUID, code string, in HandoverInput) (*Handover, error) {
	code = NormalizePickupCode(code)
	repairID, err := s.FindRepairByPickupCode(ctx, tenantID, code)
	if err != nil {
		return nil, err
	}
	in.PickupCode = code
	if strings.TrimSpace(in.CollectedByName) == "" {
		in.CollectedByName = "Customer"
	}
	if strings.TrimSpace(in.Relationship) == "" {
		in.Relationship = "self"
	}
	return s.RecordHandover(ctx, tenantID, repairID, in)
}
