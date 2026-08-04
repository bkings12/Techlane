package repair

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func normalizeAccessories(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func (s *Service) encryptPasscode(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(s.passcodeKey) == 0 {
		return nil, errors.New("device passcode encryption is not configured")
	}
	key := sha256.Sum256(s.passcodeKey)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(raw), nil), nil
}

func (s *Service) decryptPasscode(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}
	if len(s.passcodeKey) == 0 {
		return "", errors.New("device passcode encryption is not configured")
	}
	key := sha256.Sum256(s.passcodeKey)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted passcode")
	}
	plain, err := gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("could not decrypt device passcode")
	}
	return string(plain), nil
}

func intakePayload(accessories []string, condition *string) ([]byte, *string, error) {
	accessories = normalizeAccessories(accessories)
	data, err := json.Marshal(accessories)
	if err != nil {
		return nil, nil, err
	}
	if condition != nil {
		trimmed := strings.TrimSpace(*condition)
		if trimmed == "" {
			condition = nil
		} else {
			condition = &trimmed
		}
	}
	return data, condition, nil
}

// RevealDevicePasscode is intentionally a separate audited action; ordinary job
// reads expose only has_device_passcode.
func (s *Service) RevealDevicePasscode(ctx context.Context, tenantID, repairID, actorID, corrID uuid.UUID) (string, error) {
	var ciphertext []byte
	err := s.pool.QueryRow(ctx, `
		SELECT device_passcode_ciphertext
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("repair not found")
	}
	if err != nil {
		return "", err
	}
	passcode, err := s.decryptPasscode(ciphertext)
	if err != nil {
		return "", err
	}
	if passcode == "" {
		return "", errors.New("no device passcode was captured")
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events
			(id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		SELECT $1, tenant_id, id, status, 'Device passcode revealed to staff', $2, $3
		FROM repair.repair_jobs WHERE tenant_id = $4 AND id = $5`,
		uuid.New(), actorID, corrID, tenantID, repairID)
	return passcode, nil
}

// CreateRework opens a follow-up job against the same customer and device when
// a completed/collected job comes back (warranty return or come-back fix).
// The original stays the commercial record; the rework starts authorized so
// bench work is not blocked by the zero-charge default.
func (s *Service) CreateRework(ctx context.Context, tenantID, originalID uuid.UUID, reason string, actorID, corrID uuid.UUID) (*RepairJob, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("return reason required")
	}
	var branchID, deviceID uuid.UUID
	var customerID *uuid.UUID
	var originalCode, status string
	err := s.pool.QueryRow(ctx, `
		SELECT branch_id, customer_id, device_id, job_code, status
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, originalID).Scan(&branchID, &customerID, &deviceID, &originalCode, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("repair not found")
	}
	if err != nil {
		return nil, err
	}
	if status != StatusCompleted && status != StatusCollected {
		return nil, errors.New("only a completed or collected job can be opened as a return")
	}
	job, err := s.CreateRepair(ctx, CreateRepairInput{
		BranchID: branchID, CustomerID: customerID, DeviceID: deviceID,
		ProblemSummary: "Return of " + originalCode + " — " + reason,
		LaborAmount:    0, ActorID: actorID, TenantID: tenantID, CorrID: corrID,
		ParentJobID: &originalID, ReworkReason: &reason,
	})
	if err != nil {
		return nil, err
	}
	// Labor is 0 so CreateRepair did not intake-authorize; unlock the bench for the return.
	now := time.Now().UTC()
	zero := 0.0
	src := AuthSourceReturnRework
	_, _ = s.pool.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET work_authorized_at = $1,
		    work_authorization_source = $2,
		    authorized_amount = $3,
		    work_authorized_by = $4,
		    updated_at = now(),
		    version = version + 1
		WHERE tenant_id = $5 AND id = $6`,
		now, src, zero, actorID, tenantID, job.ID)
	s.publish("repair.returned", tenantID, branchID, actorID, corrID, map[string]any{
		"repair_job_id":   job.ID.String(),
		"parent_job_id":   originalID.String(),
		"parent_job_code": originalCode,
		"reason":          reason,
	})
	return s.GetRepair(ctx, tenantID, job.ID)
}

// IntakeInput is the atomic counter check-in payload: customer + device + job (+ optional estimate).
type IntakeInput struct {
	TenantID uuid.UUID
	BranchID uuid.UUID
	ActorID  uuid.UUID
	CorrID   uuid.UUID

	CustomerID    *uuid.UUID
	Anonymous     bool
	CustomerName  *string
	CustomerPhone *string

	DeviceKind   string
	Brand        *string
	Model        *string
	IMEI         *string
	SerialNumber *string

	ProblemSummary string
	ConditionTags  []string

	/**
	 * When the shop told the customer to come back. Optional — a job with no
	 * promise is simply never overdue, which is the honest default for work
	 * that has not been diagnosed yet.
	 */
	PromisedBy *time.Time

	EstimateLaborAmount *float64
	EstimatePartsAmount *float64

	TechnicianID *uuid.UUID
}

// IntakeResult is returned from a successful atomic intake.
type IntakeResult struct {
	Customer *Customer       `json:"customer,omitempty"`
	Device   *Device         `json:"device"`
	Repair   *RepairJob      `json:"repair"`
	Estimate *RepairEstimate `json:"estimate,omitempty"`
}

// Intake creates customer (optional), device, repair job, and optional estimate in one transaction.
func (s *Service) Intake(ctx context.Context, in IntakeInput) (*IntakeResult, error) {
	kind := strings.TrimSpace(strings.ToLower(in.DeviceKind))
	if kind == "" {
		kind = "phone"
	}
	switch kind {
	case "phone", "laptop", "tablet", "other":
	default:
		return nil, fmt.Errorf("device kind must be phone, laptop, tablet, or other")
	}
	problem := strings.TrimSpace(in.ProblemSummary)
	if problem == "" {
		return nil, fmt.Errorf("problem_summary required")
	}
	if in.BranchID == uuid.Nil {
		return nil, fmt.Errorf("branch_id required")
	}
	tags := normalizeAccessories(in.ConditionTags)

	var laborAmt, partsAmt float64
	if in.EstimateLaborAmount != nil {
		laborAmt = *in.EstimateLaborAmount
	}
	if in.EstimatePartsAmount != nil {
		partsAmt = *in.EstimatePartsAmount
	}
	if laborAmt < 0 || partsAmt < 0 {
		return nil, fmt.Errorf("estimate amounts cannot be negative")
	}
	estimateTotal := laborAmt + partsAmt
	// Agreed-at-counter price lives on the job as labor_amount (authorizes work).
	jobLabor := 0.0
	createEstimate := estimateTotal > 0
	if createEstimate {
		jobLabor = estimateTotal
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var customer *Customer
	var customerID *uuid.UUID
	if !in.Anonymous {
		if in.CustomerID != nil && *in.CustomerID != uuid.Nil {
			var c Customer
			err = tx.QueryRow(ctx, `
				SELECT id, full_name, phone, email FROM repair.customers
				WHERE tenant_id = $1 AND id = $2`, in.TenantID, *in.CustomerID).
				Scan(&c.ID, &c.FullName, &c.Phone, &c.Email)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("customer not found")
			}
			if err != nil {
				return nil, err
			}
			customer = &c
			customerID = &c.ID
		} else {
			name := ""
			if in.CustomerName != nil {
				name = strings.TrimSpace(*in.CustomerName)
			}
			if name == "" {
				return nil, fmt.Errorf("customer name required (or mark walk-in)")
			}
			var phonePtr *string
			var variants []string
			if in.CustomerPhone != nil {
				raw := strings.TrimSpace(*in.CustomerPhone)
				if raw != "" {
					if e164, nErr := NormalizePhone(raw); nErr == nil {
						phonePtr = &e164
						variants = PhoneMatchVariants(e164)
					} else if digits := digitsOnly(raw); digits != "" {
						phonePtr = &digits
						variants = PhoneMatchVariants(digits)
					}
				}
			}
			if len(variants) > 0 {
				var existing Customer
				qErr := tx.QueryRow(ctx, `
					SELECT c.id, c.full_name, c.phone, c.email FROM repair.customers c
					WHERE c.tenant_id = $1
					  AND regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
					ORDER BY
					  (SELECT COUNT(*) FROM repair.repair_jobs j WHERE j.customer_id = c.id) DESC,
					  c.created_at ASC
					LIMIT 1`, in.TenantID, variants).
					Scan(&existing.ID, &existing.FullName, &existing.Phone, &existing.Email)
				if qErr == nil {
					if phonePtr != nil && (existing.Phone == nil || *existing.Phone != *phonePtr) {
						_, _ = tx.Exec(ctx, `UPDATE repair.customers SET phone = $1, updated_at = now() WHERE id = $2`, *phonePtr, existing.ID)
						existing.Phone = phonePtr
					}
					customer = &existing
					customerID = &existing.ID
				} else if !errors.Is(qErr, pgx.ErrNoRows) {
					return nil, qErr
				}
			}
			if customer == nil {
				id := uuid.New()
				_, err = tx.Exec(ctx, `
					INSERT INTO repair.customers (id, tenant_id, full_name, phone, email, created_by, correlation_id)
					VALUES ($1, $2, $3, $4, NULL, $5, $6)`,
					id, in.TenantID, name, phonePtr, in.ActorID, in.CorrID)
				if err != nil {
					if len(variants) > 0 {
						var existing Customer
						if qErr := tx.QueryRow(ctx, `
							SELECT id, full_name, phone, email FROM repair.customers
							WHERE tenant_id = $1
							  AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
							ORDER BY created_at ASC LIMIT 1`, in.TenantID, variants).
							Scan(&existing.ID, &existing.FullName, &existing.Phone, &existing.Email); qErr == nil {
							customer = &existing
							customerID = &existing.ID
						} else {
							return nil, err
						}
					} else {
						return nil, err
					}
				} else {
					customer = &Customer{ID: id, FullName: name, Phone: phonePtr}
					customerID = &id
				}
			}
		}
	}

	deviceID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.devices (id, tenant_id, customer_id, anonymous, kind, brand, model, imei, serial_number, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		deviceID, in.TenantID, customerID, in.Anonymous, kind, in.Brand, in.Model, in.IMEI, in.SerialNumber, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	device := &Device{
		ID: deviceID, CustomerID: customerID, Anonymous: in.Anonymous, Kind: kind,
		Brand: in.Brand, Model: in.Model, IMEI: in.IMEI, SerialNumber: in.SerialNumber,
	}

	jobNumber, err := s.nextJobNumber(ctx, tx, in.TenantID)
	if err != nil {
		return nil, err
	}
	jobCode := FormatJobCode(jobNumber)
	pickupCode, err := s.allocatePickupCode(ctx, tx, in.TenantID)
	if err != nil {
		return nil, err
	}

	var authAt *time.Time
	var authSource *string
	var authAmount *float64
	if jobLabor > 0 {
		now := time.Now().UTC()
		src := AuthSourceIntakeAgreed
		authAt, authSource, authAmount = &now, &src, &jobLabor
	}

	var condition *string
	if len(tags) > 0 {
		joined := strings.Join(tags, ", ")
		condition = &joined
	}
	accessoriesJSON, condition, err := intakePayload(nil, condition)
	if err != nil {
		return nil, err
	}

	repairID := uuid.New()
	status := StatusIntake
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.repair_jobs (
			id, tenant_id, branch_id, customer_id, device_id, technician_id, status, problem_summary,
			labor_amount, job_number, job_code, pickup_code, created_by, updated_by, correlation_id,
			work_authorized_at, work_authorization_source, authorized_amount, work_authorized_by,
			promised_by, intake_accessories, intake_condition, device_passcode_ciphertext,
			parent_job_id, rework_reason, customer_waiting, estimated_wait_minutes, customer_credit, credit_due_date,
			service_type, condition_tags
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13, $14, $15, $16, $17,
		        CASE WHEN $15::timestamptz IS NULL THEN NULL ELSE $13::uuid END,
		        $21, $18, $19, NULL, NULL, NULL, false, NULL, false, NULL, 'repair', $20)`,
		repairID, in.TenantID, in.BranchID, customerID, deviceID, in.TechnicianID, status, problem,
		jobLabor, jobNumber, jobCode, pickupCode, in.ActorID, in.CorrID, authAt, authSource, authAmount,
		accessoriesJSON, condition, tags, in.PromisedBy)
	if err != nil {
		return nil, err
	}

	intakeNote := "Checked in — diagnose first, price to be quoted"
	if jobLabor > 0 {
		intakeNote = fmt.Sprintf("Checked in — price agreed at the counter: %.2f", jobLabor)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), in.TenantID, repairID, status, intakeNote, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}

	var estimate *RepairEstimate
	if createEstimate {
		now := time.Now().UTC()
		expires := now.Add(72 * time.Hour)
		estID := uuid.New()
		estLabor := laborAmt
		estParts := partsAmt
		if estLabor+estParts <= 0 {
			estLabor = estimateTotal
			estParts = 0
		} else if estLabor == 0 && estParts > 0 {
			estLabor = estParts
			estParts = 0
		}
		notes := "Quoted at intake"
		_, err = tx.Exec(ctx, `
			INSERT INTO repair.repair_estimates
				(id, tenant_id, repair_job_id, labor_amount, parts_amount, currency, notes, status, expires_at, created_by, created_at)
			VALUES ($1, $2, $3, $4, $5, 'KES', $6, $7, $8, $9, $10)`,
			estID, in.TenantID, repairID, estLabor, estParts, notes, EstimatePending, expires, in.ActorID, now)
		if err != nil {
			return nil, err
		}
		estimate = &RepairEstimate{
			ID: estID, RepairJobID: repairID, LaborAmount: estLabor, PartsAmount: estParts,
			TotalAmount: estLabor + estParts, Currency: "KES", Notes: &notes,
			Status: EstimatePending, ExpiresAt: &expires, CreatedBy: &in.ActorID, CreatedAt: now,
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	job := &RepairJob{
		ID: repairID, JobNumber: jobNumber, JobCode: jobCode, PickupCode: pickupCode,
		BranchID: in.BranchID, CustomerID: customerID, DeviceID: deviceID,
		TechnicianID: in.TechnicianID, Status: status, ProblemSummary: problem, ServiceType: "repair",
		LaborAmount: jobLabor, IntakeCondition: condition, ConditionTags: tags, Device: device, Customer: customer,
	}
	s.publish("repair.created", in.TenantID, in.BranchID, in.ActorID, in.CorrID, map[string]any{
		"repair_job_id": repairID.String(), "job_code": jobCode, "pickup_code": pickupCode, "status": status,
		"labor_amount": jobLabor, "problem_summary": problem, "price_agreed_at_intake": jobLabor > 0,
		"intake_atomic": true,
	})
	if estimate != nil {
		s.publish("estimate.pending", in.TenantID, in.BranchID, in.ActorID, in.CorrID, map[string]any{
			"repair_job_id": repairID.String(),
			"total_amount":  estimate.TotalAmount,
			"estimate_id":   estimate.ID.String(),
		})
	}

	return &IntakeResult{Customer: customer, Device: device, Repair: job, Estimate: estimate}, nil
}
