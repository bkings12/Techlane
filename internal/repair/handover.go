package repair

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Handover verification methods.
const (
	HandoverMethodOTP          = "otp"
	HandoverMethodPickupCode   = "pickup_code"
	HandoverMethodStaffVouched = "staff_vouched"
)

var (
	// ErrHandoverRequired is returned when something tries to mark a job collected
	// without recording who took the device.
	ErrHandoverRequired = errors.New("record the handover instead of setting collected directly")
	// ErrHandoverExists is returned when a device has already been handed over.
	ErrHandoverExists = errors.New("this device has already been handed over")
	// ErrHandoverNotReady is returned when a job is not at a stage where the
	// customer can take the device away.
	ErrHandoverNotReady = errors.New("this job is not ready for collection yet")
	// ErrHandoverVouchNotAllowed is returned when staff try to skip the code
	// without the authority to take responsibility for it.
	ErrHandoverVouchNotAllowed = errors.New("only a manager or owner can release a device without a code")
	// ErrNoCustomerPhone is returned when we cannot text a code because we never
	// captured a phone number for the owner.
	ErrNoCustomerPhone = errors.New("no phone number on record for this customer — release with a manager instead")
)

// Handover records who physically took a device and how we established they were
// entitled to it.
type Handover struct {
	ID                 uuid.UUID  `json:"id"`
	RepairJobID        uuid.UUID  `json:"repair_job_id"`
	CollectedByName    string     `json:"collected_by_name"`
	Relationship       string     `json:"relationship"`
	IDNumber           *string    `json:"id_number,omitempty"`
	Phone              *string    `json:"phone,omitempty"`
	VerificationMethod string     `json:"verification_method"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
	ReleasedBy         *uuid.UUID `json:"released_by,omitempty"`
	Note               *string    `json:"note,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// HandoverInput is what the counter fills in when releasing a device.
type HandoverInput struct {
	CollectedByName string
	Relationship    string
	IDNumber        string
	Note            string
	// OTPCode is the code the owner received by SMS. When empty the release falls
	// back to a printed pickup code, then to a staff vouch (elevated permission).
	OTPCode string
	// PickupCode is the durable claim code printed on the intake slip / QR.
	PickupCode string
	// CanVouch reports whether the actor may release without a code. Resolved from
	// permissions by the handler, so the service is not guessing at authority.
	CanVouch bool
	ActorID  uuid.UUID
	CorrID   uuid.UUID
}

// collectableStatuses are the stages a customer can take a device home from:
// QC-passed and waiting at the counter, a finished repair, or a job we are
// giving up on and handing back.
var collectableStatuses = []string{StatusReadyPickup, StatusCompleted, StatusCancelled, StatusUnrepairable}

func isCollectable(status string) bool {
	for _, s := range collectableStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// RequestHandoverOTP texts a collection code to the number on record for the job's
// customer. The code is scoped to this job so it cannot release a different device.
func (s *Service) RequestHandoverOTP(ctx context.Context, tenantID, repairID uuid.UUID) error {
	var status string
	var customerID *uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT status, customer_id FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&status, &customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("repair not found")
	}
	if err != nil {
		return err
	}
	if !isCollectable(status) {
		return ErrHandoverNotReady
	}
	if customerID == nil {
		return ErrNoCustomerPhone
	}
	var phone *string
	if err := s.pool.QueryRow(ctx, `SELECT phone FROM repair.customers WHERE id = $1`, *customerID).
		Scan(&phone); err != nil {
		return err
	}
	if phone == nil || strings.TrimSpace(*phone) == "" {
		return ErrNoCustomerPhone
	}
	phoneE164, err := NormalizePhone(*phone)
	if err != nil {
		return ErrNoCustomerPhone
	}

	now := time.Now().UTC()
	rows, err := s.pool.Query(ctx, `
		SELECT created_at, expires_at, consumed_at, attempts
		FROM repair.customer_otp_challenges
		WHERE tenant_id = $1 AND repair_job_id = $2 AND purpose = 'handover' AND created_at > $3
		ORDER BY created_at DESC LIMIT 10`, tenantID, repairID, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	defer rows.Close()
	var recent []OTPChallengeMeta
	for rows.Next() {
		var m OTPChallengeMeta
		if err := rows.Scan(&m.CreatedAt, &m.ExpiresAt, &m.ConsumedAt, &m.Attempts); err != nil {
			return err
		}
		recent = append(recent, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := CanRequestOTP(recent, now); err != nil {
		return err
	}

	code, err := generateOTPCode()
	if err != nil {
		return err
	}
	challengeID := uuid.New()
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO repair.customer_otp_challenges
			(id, tenant_id, phone_e164, code_hash, attempts, expires_at, created_at, purpose, repair_job_id)
		VALUES ($1, $2, $3, $4, 0, $5, $6, 'handover', $7)`,
		challengeID, tenantID, phoneE164, hashSecret(code), now.Add(otpTTL), now, repairID); err != nil {
		return err
	}

	var jobCode, shopName, pickupCode string
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(j.job_code, ''), LEFT(j.id::text, 8)),
		       COALESCE(NULLIF(j.pickup_code, ''), 'your intake slip'),
		       COALESCE(NULLIF(t.name, ''), 'the shop')
		FROM repair.repair_jobs j
		LEFT JOIN identity.tenants t ON t.id = j.tenant_id
		WHERE j.tenant_id = $1 AND j.id = $2`, tenantID, repairID).
		Scan(&jobCode, &pickupCode, &shopName)
	if jobCode == "" {
		jobCode = "your repair"
	}
	if shopName == "" {
		shopName = "the shop"
	}
	message := fmt.Sprintf(
		"%s: collection code for %s is %s. Give it to the attendant when collecting (or bring pickup code %s). Valid 10 minutes.",
		shopName, jobCode, code, pickupCode,
	)
	if err := s.resolveSMSSender(ctx, tenantID).SendMessage(ctx, phoneE164, message); err != nil {
		// The customer never received this code, so it must not sit there consuming
		// the retry budget and blocking the counter from trying again.
		_, _ = s.pool.Exec(ctx, `DELETE FROM repair.customer_otp_challenges WHERE id = $1`, challengeID)
		return err
	}
	return nil
}

// verifyHandoverOTP consumes a job-scoped collection code.
func (s *Service) verifyHandoverOTP(ctx context.Context, tenantID, repairID uuid.UUID, code string) error {
	code = strings.TrimSpace(code)
	if len(code) < 4 {
		return fmt.Errorf("code required")
	}
	var challengeID uuid.UUID
	var codeHash string
	var meta OTPChallengeMeta
	err := s.pool.QueryRow(ctx, `
		SELECT id, code_hash, attempts, expires_at, consumed_at, created_at
		FROM repair.customer_otp_challenges
		WHERE tenant_id = $1 AND repair_job_id = $2 AND purpose = 'handover'
		ORDER BY created_at DESC LIMIT 1`, tenantID, repairID).
		Scan(&challengeID, &codeHash, &meta.Attempts, &meta.ExpiresAt, &meta.ConsumedAt, &meta.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no active collection code; send one first")
	}
	if err != nil {
		return err
	}
	if err := CanVerifyOTP(meta, time.Now().UTC()); err != nil {
		return err
	}
	if hashSecret(code) != codeHash {
		_, _ = s.pool.Exec(ctx, `
			UPDATE repair.customer_otp_challenges SET attempts = attempts + 1 WHERE id = $1`, challengeID)
		return fmt.Errorf("invalid code")
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE repair.customer_otp_challenges SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL`,
		challengeID)
	return err
}

// RecordHandover releases a device to whoever is at the counter and moves the job
// to collected. This is the only route to collected: the status exists to mean
// "the customer has the device", so it should not be settable without saying who
// took it.
func (s *Service) RecordHandover(ctx context.Context, tenantID, repairID uuid.UUID, in HandoverInput) (*Handover, error) {
	name := strings.TrimSpace(in.CollectedByName)
	if name == "" {
		return nil, fmt.Errorf("who is collecting the device?")
	}
	relationship := strings.TrimSpace(in.Relationship)
	if relationship == "" {
		relationship = "self"
	}

	var status string
	var branchID uuid.UUID
	var storedPickup *string
	var customerCredit bool
	var creditDueDate *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id, pickup_code, customer_credit, credit_due_date
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&status, &branchID, &storedPickup, &customerCredit, &creditDueDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("repair not found")
	}
	if err != nil {
		return nil, err
	}
	if status == StatusCollected {
		return nil, ErrHandoverExists
	}
	if !isCollectable(status) {
		return nil, ErrHandoverNotReady
	}

	// Money first: handing over a device with a balance owing turns a repair into
	// an unsecured loan. Provisional counter cash counts, since that till has the
	// money even if the shift is not closed out.
	due, err := s.outstandingRepairBalance(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	if due > 0.009 && (!customerCredit || creditDueDate == nil) {
		return nil, fmt.Errorf("%w (%.2f still due)", ErrBalanceDue, due)
	}

	method := HandoverMethodOTP
	otp := strings.TrimSpace(in.OTPCode)
	pickup := NormalizePickupCode(in.PickupCode)
	switch {
	case pickup != "":
		stored := ""
		if storedPickup != nil {
			stored = NormalizePickupCode(*storedPickup)
		}
		if stored == "" || pickup != stored {
			return nil, fmt.Errorf("invalid pickup code")
		}
		method = HandoverMethodPickupCode
	case otp != "":
		if err := s.verifyHandoverOTP(ctx, tenantID, repairID, otp); err != nil {
			return nil, err
		}
		method = HandoverMethodOTP
	default:
		if !in.CanVouch {
			return nil, ErrHandoverVouchNotAllowed
		}
		method = HandoverMethodStaffVouched
	}

	now := time.Now().UTC()
	h := &Handover{
		ID: uuid.New(), RepairJobID: repairID, CollectedByName: name,
		Relationship: relationship, VerificationMethod: method,
		VerifiedAt: &now, CreatedAt: now,
	}
	if v := strings.TrimSpace(in.IDNumber); v != "" {
		h.IDNumber = &v
	}
	if v := strings.TrimSpace(in.Note); v != "" {
		h.Note = &v
	}
	if in.ActorID != uuid.Nil {
		actor := in.ActorID
		h.ReleasedBy = &actor
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO repair.job_handovers
			(id, tenant_id, repair_job_id, collected_by_name, relationship, id_number,
			 verification_method, verified_at, released_by, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		h.ID, tenantID, repairID, h.CollectedByName, h.Relationship, h.IDNumber,
		h.VerificationMethod, h.VerifiedAt, h.ReleasedBy, h.Note); err != nil {
		if strings.Contains(err.Error(), "idx_job_handovers_job") {
			return nil, ErrHandoverExists
		}
		return nil, err
	}

	// Successful repairs must pass through completed so loyalty / commission /
	// ready-SMS / warranty hooks fire even when the counter collects from
	// ready_for_pickup in one step. Closures skip completed (no repair finished).
	fromStatus := status
	if status == StatusReadyPickup {
		if _, err := tx.Exec(ctx, `
			UPDATE repair.repair_jobs SET status = $1, updated_at = now(), version = version + 1
			WHERE tenant_id = $2 AND id = $3`, StatusCompleted, tenantID, repairID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.New(), tenantID, repairID, StatusCompleted,
			"Completed at collection", in.ActorID, in.CorrID); err != nil {
			return nil, err
		}
		fromStatus = StatusCompleted
	}

	if _, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs SET status = $1, collected_at = $2, updated_at = now(), version = version + 1
		WHERE tenant_id = $3 AND id = $4`, StatusCollected, now, tenantID, repairID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairID, StatusCollected,
		handoverNote(h), in.ActorID, in.CorrID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if status == StatusReadyPickup {
		s.fireRepairCompletedSideEffects(ctx, tenantID, branchID, repairID, in.ActorID, in.CorrID)
	}
	s.maybeEnsureWarranty(ctx, tenantID, repairID, StatusCollected)
	s.publish("repair.collected", tenantID, branchID, in.ActorID, in.CorrID, map[string]any{
		"repair_job_id":       repairID.String(),
		"collected_by":        h.CollectedByName,
		"relationship":        h.Relationship,
		"verification_method": h.VerificationMethod,
		"from_status":         fromStatus,
	})
	return h, nil
}

func handoverNote(h *Handover) string {
	who := h.CollectedByName
	if h.Relationship != "self" {
		who += " (" + h.Relationship + ")"
	}
	how := "code confirmed on the owner's phone"
	switch h.VerificationMethod {
	case HandoverMethodStaffVouched:
		how = "released by staff without a code"
	case HandoverMethodPickupCode:
		how = "intake slip / QR verified"
	}
	return "Handed over to " + who + " — " + how
}

// HandoverFor returns the handover record for a job, if the device has gone out.
func (s *Service) HandoverFor(ctx context.Context, tenantID, repairID uuid.UUID) (*Handover, error) {
	var h Handover
	err := s.pool.QueryRow(ctx, `
		SELECT id, repair_job_id, collected_by_name, relationship, id_number, phone,
		       verification_method, verified_at, released_by, note, created_at
		FROM repair.job_handovers WHERE tenant_id = $1 AND repair_job_id = $2`,
		tenantID, repairID).
		Scan(&h.ID, &h.RepairJobID, &h.CollectedByName, &h.Relationship, &h.IDNumber, &h.Phone,
			&h.VerificationMethod, &h.VerifiedAt, &h.ReleasedBy, &h.Note, &h.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}
