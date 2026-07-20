package repair

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	EstimatePending  = "pending"
	EstimateApproved = "approved"
	EstimateRejected = "rejected"
	EstimateExpired  = "expired"
)

type RepairEstimate struct {
	ID           uuid.UUID  `json:"id"`
	RepairJobID  uuid.UUID  `json:"repair_job_id"`
	LaborAmount  float64    `json:"labor_amount"`
	PartsAmount  float64    `json:"parts_amount"`
	Currency     string     `json:"currency"`
	Notes        *string    `json:"notes,omitempty"`
	Status       string     `json:"status"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CreateEstimateInput struct {
	TenantID     uuid.UUID
	RepairJobID  uuid.UUID
	LaborAmount  float64
	PartsAmount  float64
	Notes        *string
	ExpiresHours *int
	ActorID      uuid.UUID
}

// CanDecideEstimate validates pending → approved/rejected transitions.
func CanDecideEstimate(status string, expiresAt *time.Time, now time.Time) error {
	if status == EstimateExpired {
		return fmt.Errorf("estimate expired")
	}
	if status != EstimatePending {
		return fmt.Errorf("estimate is %s", status)
	}
	if expiresAt != nil && !expiresAt.After(now) {
		return fmt.Errorf("estimate expired")
	}
	return nil
}

func (s *Service) CreateEstimate(ctx context.Context, in CreateEstimateInput) (*RepairEstimate, error) {
	if in.LaborAmount < 0 || in.PartsAmount < 0 {
		return nil, fmt.Errorf("amounts cannot be negative")
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2)`,
		in.TenantID, in.RepairJobID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("repair not found")
	}
	hours := 72
	if in.ExpiresHours != nil {
		if *in.ExpiresHours <= 0 || *in.ExpiresHours > 24*30 {
			return nil, fmt.Errorf("expires_hours must be between 1 and 720")
		}
		hours = *in.ExpiresHours
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(hours) * time.Hour)
	id := uuid.New()
	currency := "KES"
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.repair_estimates
			(id, tenant_id, repair_job_id, labor_amount, parts_amount, currency, notes, status, expires_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id, in.TenantID, in.RepairJobID, in.LaborAmount, in.PartsAmount, currency, in.Notes,
		EstimatePending, expires, in.ActorID, now)
	if err != nil {
		return nil, err
	}
	s.AdvanceStatusIf(ctx, in.TenantID, in.RepairJobID,
		[]string{StatusIntake}, StatusDiagnosed,
		"Estimate created", in.ActorID, uuid.New())
	s.publish("estimate.pending", in.TenantID, uuid.Nil, in.ActorID, uuid.New(), map[string]any{
		"repair_job_id": in.RepairJobID.String(),
		"labor_amount":  in.LaborAmount,
		"parts_amount":  in.PartsAmount,
		"estimate_id":   id.String(),
	})
	return &RepairEstimate{
		ID: id, RepairJobID: in.RepairJobID, LaborAmount: in.LaborAmount, PartsAmount: in.PartsAmount,
		Currency: currency, Notes: in.Notes, Status: EstimatePending, ExpiresAt: &expires,
		CreatedBy: &in.ActorID, CreatedAt: now,
	}, nil
}

func (s *Service) ListEstimates(ctx context.Context, tenantID, repairID uuid.UUID) ([]RepairEstimate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repair_job_id, labor_amount::float8, parts_amount::float8, currency, notes, status,
		       expires_at, created_by, decided_at, created_at
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2
		ORDER BY created_at DESC`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	items := make([]RepairEstimate, 0)
	for rows.Next() {
		var e RepairEstimate
		if err := rows.Scan(
			&e.ID, &e.RepairJobID, &e.LaborAmount, &e.PartsAmount, &e.Currency, &e.Notes,
			&e.Status, &e.ExpiresAt, &e.CreatedBy, &e.DecidedAt, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		if e.Status == EstimatePending && e.ExpiresAt != nil && !e.ExpiresAt.After(now) {
			e.Status = EstimateExpired
			_, _ = s.pool.Exec(ctx, `
				UPDATE repair.repair_estimates SET status = $1 WHERE id = $2 AND status = $3`,
				EstimateExpired, e.ID, EstimatePending)
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (s *Service) decideEstimate(ctx context.Context, tenantID, repairID, estimateID, customerID uuid.UUID, decision string) (*RepairEstimate, error) {
	if err := s.AssertCustomerOwnsRepair(ctx, tenantID, customerID, repairID); err != nil {
		return nil, err
	}
	var e RepairEstimate
	err := s.pool.QueryRow(ctx, `
		SELECT id, repair_job_id, labor_amount::float8, parts_amount::float8, currency, notes, status,
		       expires_at, created_by, decided_at, created_at
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, estimateID).
		Scan(&e.ID, &e.RepairJobID, &e.LaborAmount, &e.PartsAmount, &e.Currency, &e.Notes,
			&e.Status, &e.ExpiresAt, &e.CreatedBy, &e.DecidedAt, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("estimate not found")
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := CanDecideEstimate(e.Status, e.ExpiresAt, now); err != nil {
		if e.Status == EstimatePending && e.ExpiresAt != nil && !e.ExpiresAt.After(now) {
			_, _ = s.pool.Exec(ctx, `UPDATE repair.repair_estimates SET status = $1 WHERE id = $2`, EstimateExpired, e.ID)
		}
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		UPDATE repair.repair_estimates SET status = $1, decided_at = $2 WHERE id = $3 AND status = $4`,
		decision, now, e.ID, EstimatePending)
	if err != nil {
		return nil, err
	}
	if decision == EstimateApproved {
		_, err = tx.Exec(ctx, `
			UPDATE repair.repair_jobs SET labor_amount = $1, updated_at = now(), version = version + 1
			WHERE tenant_id = $2 AND id = $3`, e.LaborAmount, tenantID, repairID)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	e.Status = decision
	e.DecidedAt = &now
	if decision == EstimateApproved {
		// Customer approved work — move into active repair unless waiting on parts.
		s.AdvanceStatusIf(ctx, tenantID, repairID,
			[]string{StatusIntake, StatusDiagnosed}, StatusInProgress,
			"Customer approved estimate", customerID, uuid.New())
	}
	return &e, nil
}

func (s *Service) ApproveEstimate(ctx context.Context, tenantID, repairID, estimateID, customerID uuid.UUID) (*RepairEstimate, error) {
	return s.decideEstimate(ctx, tenantID, repairID, estimateID, customerID, EstimateApproved)
}

func (s *Service) RejectEstimate(ctx context.Context, tenantID, repairID, estimateID, customerID uuid.UUID) (*RepairEstimate, error) {
	return s.decideEstimate(ctx, tenantID, repairID, estimateID, customerID, EstimateRejected)
}
