package identity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CommissionConfigInput struct {
	CommissionEnabled bool     `json:"commission_enabled"`
	CommissionType    string   `json:"commission_type"` // none | percent_of_job | fixed_per_job
	PercentBPS        *int     `json:"percent_bps"`
	FixedAmount       *float64 `json:"fixed_amount"`
}

type CommissionEntry struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	BranchID         *uuid.UUID `json:"branch_id,omitempty"`
	UserID           uuid.UUID  `json:"user_id"`
	RepairJobID      uuid.UUID  `json:"repair_job_id"`
	EntryType        string     `json:"entry_type"`
	BaseAmount       float64    `json:"base_amount"`
	CommissionAmount float64    `json:"commission_amount"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	TechnicianName   string     `json:"technician_name,omitempty"`
}

func (s *Service) SetCommission(ctx context.Context, tenantID, userID, actorID uuid.UUID, in CommissionConfigInput) (*StaffUser, error) {
	if actorID == userID {
		return nil, fmt.Errorf("%w: cannot edit own commission", ErrForbidden)
	}
	switch in.CommissionType {
	case "none", "percent_of_job", "fixed_per_job":
	default:
		return nil, fmt.Errorf("%w: invalid commission_type", ErrInvalidInput)
	}
	if !in.CommissionEnabled {
		in.CommissionType = "none"
	}
	if in.CommissionType == "percent_of_job" {
		if in.PercentBPS == nil || *in.PercentBPS < 0 || *in.PercentBPS > 10000 {
			return nil, fmt.Errorf("%w: percent_bps required (0-10000)", ErrInvalidInput)
		}
	}
	if in.CommissionType == "fixed_per_job" {
		if in.FixedAmount == nil || *in.FixedAmount < 0 {
			return nil, fmt.Errorf("%w: fixed_amount required", ErrInvalidInput)
		}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity.employee_profiles (
			user_id, tenant_id, commission_enabled, commission_type, percent_bps, fixed_amount, updated_by, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (user_id) DO UPDATE SET
			commission_enabled = EXCLUDED.commission_enabled,
			commission_type = EXCLUDED.commission_type,
			percent_bps = EXCLUDED.percent_bps,
			fixed_amount = EXCLUDED.fixed_amount,
			updated_by = EXCLUDED.updated_by,
			updated_at = now()`,
		userID, tenantID, in.CommissionEnabled, in.CommissionType, in.PercentBPS, in.FixedAmount, actorID)
	if err != nil {
		return nil, err
	}
	return s.GetUser(ctx, tenantID, userID)
}

// AccrueOnRepairCompleted creates a pending commission accrual when enabled for the technician.
// Idempotent per (tenant, repair, user, accrual).
func (s *Service) AccrueOnRepairCompleted(
	ctx context.Context,
	tenantID, branchID, repairJobID, technicianID uuid.UUID,
	laborAmount float64,
	actorID, corrID uuid.UUID,
) (*CommissionEntry, error) {
	if technicianID == uuid.Nil {
		return nil, nil
	}
	var enabled bool
	var ctype string
	var percentBPS *int
	var fixedAmount *float64
	err := s.pool.QueryRow(ctx, `
		SELECT commission_enabled, commission_type, percent_bps, fixed_amount
		FROM identity.employee_profiles WHERE user_id = $1 AND tenant_id = $2`,
		technicianID, tenantID).Scan(&enabled, &ctype, &percentBPS, &fixedAmount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !enabled || ctype == "none" {
		return nil, nil
	}

	amount := 0.0
	switch ctype {
	case "percent_of_job":
		bps := 0
		if percentBPS != nil {
			bps = *percentBPS
		}
		amount = math.Round(laborAmount*float64(bps)/10000*100) / 100
	case "fixed_per_job":
		if fixedAmount != nil {
			amount = *fixedAmount
		}
	default:
		return nil, nil
	}
	if amount <= 0 {
		return nil, nil
	}

	id := uuid.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO identity.commission_entries (
			id, tenant_id, branch_id, user_id, repair_job_id, entry_type,
			base_amount, commission_amount, currency, status, created_by, correlation_id
		) VALUES ($1,$2,$3,$4,$5,'accrual',$6,$7,'KES','pending',$8,$9)
		ON CONFLICT (tenant_id, repair_job_id, user_id, entry_type) DO NOTHING`,
		id, tenantID, branchID, technicianID, repairJobID, laborAmount, amount, actorID, corrID)
	if err != nil {
		return nil, err
	}

	var entry CommissionEntry
	err = s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, branch_id, user_id, repair_job_id, entry_type,
		       base_amount::float8, commission_amount::float8, currency, status, created_at
		FROM identity.commission_entries
		WHERE tenant_id = $1 AND repair_job_id = $2 AND user_id = $3 AND entry_type = 'accrual'`,
		tenantID, repairJobID, technicianID).
		Scan(&entry.ID, &entry.TenantID, &entry.BranchID, &entry.UserID, &entry.RepairJobID,
			&entry.EntryType, &entry.BaseAmount, &entry.CommissionAmount, &entry.Currency, &entry.Status, &entry.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *Service) ListCommissions(ctx context.Context, tenantID uuid.UUID, status string, userID *uuid.UUID) ([]CommissionEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.tenant_id, c.branch_id, c.user_id, c.repair_job_id, c.entry_type,
		       c.base_amount::float8, c.commission_amount::float8, c.currency, c.status, c.created_at,
		       u.display_name
		FROM identity.commission_entries c
		JOIN identity.users u ON u.id = c.user_id
		WHERE c.tenant_id = $1
		  AND ($2 = '' OR c.status = $2)
		  AND ($3::uuid IS NULL OR c.user_id = $3)
		ORDER BY c.created_at DESC
		LIMIT 200`, tenantID, status, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CommissionEntry
	for rows.Next() {
		var e CommissionEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.BranchID, &e.UserID, &e.RepairJobID, &e.EntryType,
			&e.BaseAmount, &e.CommissionAmount, &e.Currency, &e.Status, &e.CreatedAt, &e.TechnicianName); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

func (s *Service) ApproveCommission(ctx context.Context, tenantID, entryID, actorID uuid.UUID) (*CommissionEntry, error) {
	return s.setCommissionStatus(ctx, tenantID, entryID, actorID, "pending", "approved")
}

func (s *Service) MarkCommissionPaid(ctx context.Context, tenantID, entryID, actorID uuid.UUID) (*CommissionEntry, error) {
	ct, err := s.pool.Exec(ctx, `
		UPDATE identity.commission_entries SET status = 'paid'
		WHERE tenant_id = $1 AND id = $2 AND status IN ('pending', 'approved')`, tenantID, entryID)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	_ = actorID
	return s.getCommission(ctx, tenantID, entryID)
}

func (s *Service) setCommissionStatus(ctx context.Context, tenantID, entryID, actorID uuid.UUID, from, to string) (*CommissionEntry, error) {
	ct, err := s.pool.Exec(ctx, `
		UPDATE identity.commission_entries SET status = $1
		WHERE tenant_id = $2 AND id = $3 AND status = $4`, to, tenantID, entryID, from)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	_ = actorID
	return s.getCommission(ctx, tenantID, entryID)
}

func (s *Service) getCommission(ctx context.Context, tenantID, entryID uuid.UUID) (*CommissionEntry, error) {
	var e CommissionEntry
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.branch_id, c.user_id, c.repair_job_id, c.entry_type,
		       c.base_amount::float8, c.commission_amount::float8, c.currency, c.status, c.created_at,
		       u.display_name
		FROM identity.commission_entries c
		JOIN identity.users u ON u.id = c.user_id
		WHERE c.tenant_id = $1 AND c.id = $2`, tenantID, entryID).
		Scan(&e.ID, &e.TenantID, &e.BranchID, &e.UserID, &e.RepairJobID, &e.EntryType,
			&e.BaseAmount, &e.CommissionAmount, &e.Currency, &e.Status, &e.CreatedAt, &e.TechnicianName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ComputeCommissionAmount is exported for unit tests.
func ComputeCommissionAmount(ctype string, laborAmount float64, percentBPS *int, fixedAmount *float64) float64 {
	switch ctype {
	case "percent_of_job":
		bps := 0
		if percentBPS != nil {
			bps = *percentBPS
		}
		return math.Round(laborAmount*float64(bps)/10000*100) / 100
	case "fixed_per_job":
		if fixedAmount != nil {
			return *fixedAmount
		}
	}
	return 0
}
