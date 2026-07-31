package repair

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Sources of work authorisation, in descending order of strength.
const (
	AuthSourceCustomerEstimate = "customer_estimate"
	// AuthSourceIntakeAgreed is a price agreed face to face at the counter during
	// intake. The customer is present, so front-desk staff can record it.
	AuthSourceIntakeAgreed    = "intake_agreed"
	AuthSourceManagerOverride = "manager_override"
	// AuthSourceReturnRework is a zero-charge (or follow-up) job opened when a
	// customer brings a previously completed/collected device back.
	AuthSourceReturnRework = "return_rework"
	AuthSourceLegacy       = "legacy"
)

// varianceTolerance is the amount by which the final labour charge may exceed the
// authorised price before a written reason is required. Set to a shilling so it
// only absorbs rounding, not judgement calls.
const varianceTolerance = 1.0

var (
	// ErrWorkNotAuthorized blocks bench work on a job nobody has agreed to pay for.
	ErrWorkNotAuthorized = errors.New("work not authorized: the customer must approve an estimate, or a manager must record a go-ahead")
	// ErrVarianceReasonRequired blocks charging more than the customer agreed to without an explanation.
	ErrVarianceReasonRequired = errors.New("final charge exceeds the authorized amount — a reason is required")
	// ErrPartsOutstanding blocks finishing (or leaving waiting_parts) while supplier parts are open.
	ErrPartsOutstanding = errors.New("parts are still pending with the supplier — collect or cancel them before finishing")
	// ErrEstimatePending blocks finishing while a customer quote is still awaiting a decision.
	ErrEstimatePending = errors.New("a customer estimate is still pending — wait for approval or cancel it before finishing")
)

// WorkAuthorization is the record of who agreed to the price and when.
type WorkAuthorization struct {
	AuthorizedAt     *time.Time `json:"authorized_at,omitempty"`
	AuthorizedBy     *uuid.UUID `json:"authorized_by,omitempty"`
	Source           string     `json:"source,omitempty"`
	AuthorizedAmount *float64   `json:"authorized_amount,omitempty"`
	VarianceReason   *string    `json:"variance_reason,omitempty"`
}

// AuthorizeWork records a manager's go-ahead for a job with no approved estimate.
// This is the walk-in path: the customer is standing at the counter and says "just
// fix it". It is deliberately an explicit, permissioned, audited action rather than
// a silent default, because it is the one way to bypass the customer's written
// approval.
func (s *Service) AuthorizeWork(ctx context.Context, tenantID, repairID uuid.UUID, amount *float64, note string, actorID, corrID uuid.UUID) (*RepairJob, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, fmt.Errorf("a note is required when authorizing work without a customer-approved estimate")
	}
	if amount != nil && *amount < 0 {
		return nil, fmt.Errorf("authorized amount cannot be negative")
	}

	var status string
	var branchID uuid.UUID
	var currentLabor float64
	var existingAuth *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id, COALESCE(labor_amount, 0)::float8, work_authorized_at
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, repairID).
		Scan(&status, &branchID, &currentLabor, &existingAuth)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	if IsClosure(status) || status == StatusCollected {
		return nil, fmt.Errorf("cannot authorize work on a job that is already %s", status)
	}
	var hasPayment bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM payments.payment_allocations a
			WHERE a.tenant_id = $1 AND a.payable_type = 'repair' AND a.payable_id = $2
		)`, tenantID, repairID).Scan(&hasPayment); err != nil {
		return nil, err
	}
	if hasPayment {
		return nil, fmt.Errorf("agreed amount cannot be changed after payment has started; refund or reverse the payment first")
	}

	agreed := currentLabor
	if amount != nil {
		agreed = *amount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET work_authorized_at = now(), work_authorized_by = $1, work_authorization_source = $2,
		    authorized_amount = $3, labor_amount = $3, updated_by = $1, updated_at = now(), version = version + 1
		WHERE tenant_id = $4 AND id = $5`,
		actorID, AuthSourceManagerOverride, agreed, tenantID, repairID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairID, status,
		fmt.Sprintf("Agreed amount recorded/corrected by staff to %.2f — %s", agreed, note), actorID, corrID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.publish("repair.work_authorized", tenantID, branchID, actorID, corrID, map[string]any{
		"repair_job_id":     repairID.String(),
		"authorized_amount": agreed,
		"source":            AuthSourceManagerOverride,
		"note":              note,
	})
	return s.GetRepair(ctx, tenantID, repairID)
}

// AcceptPaidAsCharge writes the job charge down to what has already been paid so
// the remaining balance clears. Used when staff collected less than the original
// quote (customer paid KES 2,600 on a KES 2,700 card) and do not want another STK.
func (s *Service) AcceptPaidAsCharge(ctx context.Context, tenantID, repairID uuid.UUID, actorID, corrID uuid.UUID) (*RepairJob, error) {
	var status string
	var branchID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&status, &branchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	if IsClosure(status) || status == StatusCollected {
		return nil, fmt.Errorf("cannot adjust charge on a job that is already %s", status)
	}

	_, balance, total, _, _, err := s.repairPaymentAmounts(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	paid := total - balance
	if paid <= 0 {
		return nil, fmt.Errorf("no successful payment on this job yet")
	}
	if balance <= varianceTolerance {
		return s.GetRepair(ctx, tenantID, repairID)
	}

	saleExtra, err := s.saleLinesTotal(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	newLabor := paid - saleExtra
	if newLabor < 0 {
		newLabor = 0
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET labor_amount = $1, authorized_amount = $1, updated_by = $2, updated_at = now(), version = version + 1
		WHERE tenant_id = $3 AND id = $4`,
		newLabor, actorID, tenantID, repairID); err != nil {
		return nil, err
	}
	note := fmt.Sprintf(
		"Charge adjusted to payments received (%.2f). Shortfall of %.2f accepted — no further collection.",
		paid, balance,
	)
	if _, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairID, status, note, actorID, corrID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.publish("repair.charge_adjusted", tenantID, branchID, actorID, corrID, map[string]any{
		"repair_job_id": repairID.String(),
		"labor_amount":  newLabor,
		"paid_total":    paid,
		"shortfall":     balance,
	})
	return s.GetRepair(ctx, tenantID, repairID)
}

// loadAuthorization reads the authorisation state for gating checks.
func (s *Service) loadAuthorization(ctx context.Context, tenantID, repairID uuid.UUID) (*WorkAuthorization, error) {
	var a WorkAuthorization
	var source *string
	err := s.pool.QueryRow(ctx, `
		SELECT work_authorized_at, work_authorized_by, work_authorization_source,
		       authorized_amount::float8, labor_variance_reason
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, repairID).
		Scan(&a.AuthorizedAt, &a.AuthorizedBy, &source, &a.AuthorizedAmount, &a.VarianceReason)
	if err != nil {
		return nil, err
	}
	if source != nil {
		a.Source = *source
	}
	return &a, nil
}

// hasApprovedEstimate reports whether the customer has approved a price in writing.
func (s *Service) hasApprovedEstimate(ctx context.Context, tenantID, repairID uuid.UUID) (bool, float64) {
	var labor, parts float64
	err := s.pool.QueryRow(ctx, `
		SELECT labor_amount::float8, parts_amount::float8
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status = $3
		ORDER BY decided_at DESC NULLS LAST LIMIT 1`,
		tenantID, repairID, EstimateApproved).Scan(&labor, &parts)
	if err != nil {
		return false, 0
	}
	return true, labor + parts
}

// exceedsAuthorizedAmount reports whether final is materially above the agreed price.
func exceedsAuthorizedAmount(authorized *float64, final float64) bool {
	if authorized == nil {
		return false
	}
	return final-*authorized > varianceTolerance
}

// LaborVariance returns how far the final charge drifted from the agreed price.
func LaborVariance(authorized *float64, final float64) float64 {
	if authorized == nil {
		return 0
	}
	v := final - *authorized
	if math.Abs(v) < 0.005 {
		return 0
	}
	return v
}
