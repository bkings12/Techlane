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
	ID          uuid.UUID  `json:"id"`
	RepairJobID uuid.UUID  `json:"repair_job_id"`
	LaborAmount float64    `json:"-"` // internal — customers only see TotalAmount
	PartsAmount float64    `json:"-"`
	TotalAmount float64    `json:"total_amount"`
	Currency    string     `json:"currency"`
	Notes       *string    `json:"notes,omitempty"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateEstimateInput struct {
	TenantID     uuid.UUID
	RepairJobID  uuid.UUID
	// TotalAmount is what the customer is quoted. Labor/Parts remain internal
	// bookkeeping (optional split); prefer TotalAmount from the counter UI.
	TotalAmount  float64
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
	labor, parts := in.LaborAmount, in.PartsAmount
	if in.TotalAmount > 0 {
		// Customer-facing quote is a single total; keep the split internal as labour.
		labor, parts = in.TotalAmount, 0
	}
	if labor < 0 || parts < 0 {
		return nil, fmt.Errorf("amounts cannot be negative")
	}
	if labor+parts <= 0 {
		return nil, fmt.Errorf("estimate total must be greater than zero")
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
		id, in.TenantID, in.RepairJobID, labor, parts, currency, in.Notes,
		EstimatePending, expires, in.ActorID, now)
	if err != nil {
		return nil, err
	}
	s.AdvanceStatusIf(ctx, in.TenantID, in.RepairJobID,
		[]string{StatusIntake}, StatusDiagnosed,
		"Estimate created", in.ActorID, uuid.New())
	total := labor + parts
	s.publish("estimate.pending", in.TenantID, uuid.Nil, in.ActorID, uuid.New(), map[string]any{
		"repair_job_id": in.RepairJobID.String(),
		"total_amount":  total,
		"estimate_id":   id.String(),
	})
	return &RepairEstimate{
		ID: id, RepairJobID: in.RepairJobID, LaborAmount: labor, PartsAmount: parts, TotalAmount: total,
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
		e.TotalAmount = e.LaborAmount + e.PartsAmount
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
		// The customer agreeing to a price *is* the authorisation to do the work.
		total := e.LaborAmount + e.PartsAmount
		_, err = tx.Exec(ctx, `
			UPDATE repair.repair_jobs
			SET labor_amount = $1, work_authorized_at = $2, work_authorized_by = NULL,
			    work_authorization_source = $3, authorized_amount = $1,
			    updated_at = now(), version = version + 1
			WHERE tenant_id = $4 AND id = $5`,
			total, now, AuthSourceCustomerEstimate, tenantID, repairID)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	e.Status = decision
	e.DecidedAt = &now
	e.TotalAmount = e.LaborAmount + e.PartsAmount
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

// DecideEstimateByPhone approves or rejects a pending estimate for the customer
// who owns the phone (WhatsApp / two-way messaging). No portal JWT required.
// If estimateID is nil, the latest pending estimate for that customer is used.
func (s *Service) DecideEstimateByPhone(ctx context.Context, tenantID uuid.UUID, phone string, approve bool, estimateID *uuid.UUID) (jobCode string, err error) {
	e164, err := NormalizePhone(phone)
	if err != nil {
		return "", fmt.Errorf("invalid phone")
	}
	customer, err := s.findCustomerByPhoneVariants(ctx, tenantID, PhoneMatchVariants(e164))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no customer found for this number")
	}
	if err != nil {
		return "", err
	}
	var repairID, estID uuid.UUID
	var code string
	if estimateID != nil && *estimateID != uuid.Nil {
		err = s.pool.QueryRow(ctx, `
			SELECT e.id, e.repair_job_id, COALESCE(j.job_code, '')
			FROM repair.repair_estimates e
			JOIN repair.repair_jobs j ON j.id = e.repair_job_id AND j.tenant_id = e.tenant_id
			WHERE e.tenant_id = $1 AND e.id = $2 AND e.status = $3 AND j.customer_id = $4`,
			tenantID, *estimateID, EstimatePending, customer.ID).
			Scan(&estID, &repairID, &code)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT e.id, e.repair_job_id, COALESCE(j.job_code, '')
			FROM repair.repair_estimates e
			JOIN repair.repair_jobs j ON j.id = e.repair_job_id AND j.tenant_id = e.tenant_id
			WHERE e.tenant_id = $1 AND e.status = $2 AND j.customer_id = $3
			ORDER BY e.created_at DESC
			LIMIT 1`, tenantID, EstimatePending, customer.ID).
			Scan(&estID, &repairID, &code)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no pending estimate for this number")
	}
	if err != nil {
		return "", err
	}
	decision := EstimateRejected
	if approve {
		decision = EstimateApproved
	}
	if _, err := s.decideEstimate(ctx, tenantID, repairID, estID, customer.ID, decision); err != nil {
		return "", err
	}
	return code, nil
}
