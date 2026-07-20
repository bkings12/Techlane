package repair

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const DefaultWarrantyDays = 90

const (
	WarrantyActive  = "active"
	WarrantyClaimed = "claimed"
	WarrantyExpired = "expired"
)

type Warranty struct {
	ID           uuid.UUID  `json:"id"`
	RepairJobID  uuid.UUID  `json:"repair_job_id"`
	StartsAt     time.Time  `json:"starts_at"`
	EndsAt       time.Time  `json:"ends_at"`
	DurationDays int        `json:"duration_days"`
	Status       string     `json:"status"`
	ClaimNote    *string    `json:"claim_note,omitempty"`
	ClaimedAt    *time.Time `json:"claimed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CreateWarrantyInput struct {
	TenantID     uuid.UUID
	RepairJobID  uuid.UUID
	DurationDays *int
}

func warrantyEndsAt(start time.Time, days int) time.Time {
	return start.AddDate(0, 0, days)
}

func effectiveWarrantyStatus(status string, endsAt, now time.Time) string {
	if status == WarrantyClaimed {
		return WarrantyClaimed
	}
	if now.After(endsAt) {
		return WarrantyExpired
	}
	return WarrantyActive
}

// EnsureWarranty creates a default warranty when a repair is completed or collected.
func (s *Service) EnsureWarranty(ctx context.Context, tenantID, repairID uuid.UUID) (*Warranty, error) {
	existing, err := s.GetWarranty(ctx, tenantID, repairID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrWarrantyNotFound) {
		return nil, err
	}
	return s.createWarranty(ctx, CreateWarrantyInput{
		TenantID: tenantID, RepairJobID: repairID,
	})
}

func (s *Service) CreateWarranty(ctx context.Context, in CreateWarrantyInput) (*Warranty, error) {
	if _, err := s.GetWarranty(ctx, in.TenantID, in.RepairJobID); err == nil {
		return nil, fmt.Errorf("warranty already exists")
	} else if !errors.Is(err, ErrWarrantyNotFound) {
		return nil, err
	}
	return s.createWarranty(ctx, in)
}

func (s *Service) createWarranty(ctx context.Context, in CreateWarrantyInput) (*Warranty, error) {
	var status string
	if err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		in.TenantID, in.RepairJobID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	if status != StatusCompleted && status != StatusCollected {
		return nil, fmt.Errorf("warranty only available for completed or collected repairs")
	}
	days := DefaultWarrantyDays
	if in.DurationDays != nil && *in.DurationDays > 0 {
		days = *in.DurationDays
	}
	now := time.Now().UTC()
	id := uuid.New()
	endsAt := warrantyEndsAt(now, days)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.warranties
			(id, tenant_id, repair_job_id, starts_at, ends_at, duration_days, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, in.TenantID, in.RepairJobID, now, endsAt, days, WarrantyActive, now)
	if err != nil {
		return nil, err
	}
	return &Warranty{
		ID: id, RepairJobID: in.RepairJobID, StartsAt: now, EndsAt: endsAt,
		DurationDays: days, Status: WarrantyActive, CreatedAt: now,
	}, nil
}

var ErrWarrantyNotFound = errors.New("warranty not found")

func (s *Service) GetWarranty(ctx context.Context, tenantID, repairID uuid.UUID) (*Warranty, error) {
	var w Warranty
	var claimNote *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, repair_job_id, starts_at, ends_at, duration_days, status, claim_note, claimed_at, created_at
		FROM repair.warranties
		WHERE tenant_id = $1 AND repair_job_id = $2`, tenantID, repairID).
		Scan(&w.ID, &w.RepairJobID, &w.StartsAt, &w.EndsAt, &w.DurationDays, &w.Status, &claimNote, &w.ClaimedAt, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWarrantyNotFound
	}
	if err != nil {
		return nil, err
	}
	w.ClaimNote = claimNote
	w.Status = effectiveWarrantyStatus(w.Status, w.EndsAt, time.Now().UTC())
	return &w, nil
}

type ClaimWarrantyInput struct {
	TenantID    uuid.UUID
	RepairJobID uuid.UUID
	CustomerID  *uuid.UUID
	Note        string
}

func (s *Service) ClaimWarranty(ctx context.Context, in ClaimWarrantyInput) (*Warranty, error) {
	if in.CustomerID != nil {
		if err := s.AssertCustomerOwnsRepair(ctx, in.TenantID, *in.CustomerID, in.RepairJobID); err != nil {
			return nil, err
		}
	}
	w, err := s.GetWarranty(ctx, in.TenantID, in.RepairJobID)
	if err != nil {
		return nil, err
	}
	if w.Status == WarrantyClaimed {
		return nil, fmt.Errorf("warranty already claimed")
	}
	if w.Status == WarrantyExpired {
		return nil, fmt.Errorf("warranty expired")
	}
	now := time.Now().UTC()
	note := in.Note
	_, err = s.pool.Exec(ctx, `
		UPDATE repair.warranties
		SET status = $1, claim_note = $2, claimed_at = $3
		WHERE tenant_id = $4 AND repair_job_id = $5`,
		WarrantyClaimed, nullIfEmptyStr(note), now, in.TenantID, in.RepairJobID)
	if err != nil {
		return nil, err
	}
	w.Status = WarrantyClaimed
	w.ClaimNote = &note
	w.ClaimedAt = &now
	return w, nil
}

func nullIfEmptyStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) maybeEnsureWarranty(ctx context.Context, tenantID, repairID uuid.UUID, status string) {
	if status != StatusCompleted && status != StatusCollected {
		return
	}
	_, _ = s.EnsureWarranty(ctx, tenantID, repairID)
}
