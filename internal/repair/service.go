package repair

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/receipts"
	"github.com/techlane/techlane/packages/pkg/events"
	"github.com/techlane/techlane/packages/pkg/objectstore"
)

type Service struct {
	pool           *pgxpool.Pool
	bus            *events.Bus
	sms            SMSSender
	store          *objectstore.Store
	commissionHook CommissionHook
	completionHook CompletionHook
	passcodeKey    []byte
	stock          StockDeductor
}

// CommissionHook accrues technician commission when a repair is completed.
type CommissionHook interface {
	AccrueOnRepairCompleted(ctx context.Context, tenantID, branchID, repairJobID, technicianID uuid.UUID, laborAmount float64, actorID, corrID uuid.UUID) error
}

// CompletionHook runs after repair status changes (clear risk alerts).
type CompletionHook interface {
	OnRepairCompleted(ctx context.Context, tenantID, repairJobID, actorID uuid.UUID) error
	OnRepairStatusChanged(ctx context.Context, tenantID, repairJobID uuid.UUID, newStatus string, actorID uuid.UUID) error
}

func NewService(pool *pgxpool.Pool, bus *events.Bus) *Service {
	return &Service{pool: pool, bus: bus, sms: NoopSMSSender{}}
}

func (s *Service) SetSMSSender(sender SMSSender) {
	if sender != nil {
		s.sms = sender
	}
}

func (s *Service) SetObjectStore(store *objectstore.Store) {
	s.store = store
}

func (s *Service) SetCommissionHook(h CommissionHook) {
	s.commissionHook = h
}

func (s *Service) SetCompletionHook(h CompletionHook) {
	s.completionHook = h
}

func (s *Service) SetPasscodeKey(key []byte) {
	s.passcodeKey = append([]byte(nil), key...)
}

type Customer struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
	Phone    *string   `json:"phone,omitempty"`
	Email    *string   `json:"email,omitempty"`
}

type Device struct {
	ID           uuid.UUID  `json:"id"`
	CustomerID   *uuid.UUID `json:"customer_id,omitempty"`
	Anonymous    bool       `json:"anonymous"`
	Kind         string     `json:"kind"`
	Brand        *string    `json:"brand,omitempty"`
	Model        *string    `json:"model,omitempty"`
	IMEI         *string    `json:"imei,omitempty"`
	SerialNumber *string    `json:"serial_number,omitempty"`
}

type StatusEvent struct {
	Status string     `json:"status"`
	Note   *string    `json:"note,omitempty"`
	At     time.Time  `json:"at"`
	By     *uuid.UUID `json:"by,omitempty"`
}

type RepairJob struct {
	ID             uuid.UUID     `json:"id"`
	JobNumber      int           `json:"job_number"`
	JobCode        string        `json:"job_code"`
	PickupCode     string        `json:"pickup_code,omitempty"`
	BranchID       uuid.UUID     `json:"branch_id"`
	CustomerID     *uuid.UUID    `json:"customer_id,omitempty"`
	CustomerName   *string       `json:"customer_name,omitempty"`
	DeviceID       uuid.UUID     `json:"device_id"`
	TechnicianID   *uuid.UUID    `json:"technician_id,omitempty"`
	Status         string        `json:"status"`
	ProblemSummary string        `json:"problem_summary"`
	ServiceType    string        `json:"service_type"`
	LaborAmount    float64       `json:"labor_amount"`
	SaleLinesTotal float64       `json:"sale_lines_total"`
	SaleLines      []JobSaleLine `json:"sale_lines,omitempty"`
	// List/board money rollups — detail page also recomputes from estimates/payments.
	AuthorizedAmount      *float64           `json:"authorized_amount,omitempty"`
	ApprovedEstimateTotal *float64           `json:"approved_estimate_total,omitempty"`
	PendingEstimateTotal  *float64           `json:"pending_estimate_total,omitempty"`
	PaidTotal             float64            `json:"paid_total"`
	AmountDue             float64            `json:"amount_due"`
	BalanceDue            float64            `json:"balance_due"`
	QuotedValue           float64            `json:"quoted_value"` // best known job worth for pipeline views
	CreatedAt             time.Time          `json:"created_at"`
	PromisedBy            *time.Time         `json:"promised_by,omitempty"`
	CustomerWaiting       bool               `json:"customer_waiting"`
	EstimatedWaitMin      *int               `json:"estimated_wait_minutes,omitempty"`
	CustomerCredit        bool               `json:"customer_credit"`
	CreditDueDate         *time.Time         `json:"credit_due_date,omitempty"`
	IntakeAccessories     []string           `json:"intake_accessories,omitempty"`
	IntakeCondition       *string            `json:"intake_condition,omitempty"`
	ConditionTags         []string           `json:"condition_tags,omitempty"`
	HasDevicePasscode     bool               `json:"has_device_passcode"`
	ParentJobID           *uuid.UUID         `json:"parent_job_id,omitempty"`
	ParentJobCode         *string            `json:"parent_job_code,omitempty"`
	ReworkReason          *string            `json:"rework_reason,omitempty"`
	DeletedAt             *time.Time         `json:"deleted_at,omitempty"`
	DeletedBy             *uuid.UUID         `json:"deleted_by,omitempty"`
	ClosureReason         *string            `json:"closure_reason,omitempty"`
	ClosedAt              *time.Time         `json:"closed_at,omitempty"`
	Version               int                `json:"version"`
	Authorization         *WorkAuthorization `json:"authorization,omitempty"`
	Customer              *Customer          `json:"customer,omitempty"`
	Device                *Device            `json:"device,omitempty"`
	Timeline              []StatusEvent      `json:"timeline,omitempty"`
}

// applyJobMoney fills amount_due / balance_due / quoted_value from labor, estimates, and sales.
func applyJobMoney(j *RepairJob) {
	charge := j.LaborAmount
	if j.ApprovedEstimateTotal != nil && *j.ApprovedEstimateTotal > 0 {
		charge = *j.ApprovedEstimateTotal
	}
	j.AmountDue = charge + j.SaleLinesTotal
	if j.AmountDue < 0 {
		j.AmountDue = 0
	}
	j.BalanceDue = j.AmountDue - j.PaidTotal
	if j.BalanceDue < 0 {
		j.BalanceDue = 0
	}
	quoted := charge
	if quoted <= 0 && j.AuthorizedAmount != nil {
		quoted = *j.AuthorizedAmount
	}
	if quoted <= 0 && j.PendingEstimateTotal != nil {
		quoted = *j.PendingEstimateTotal
	}
	j.QuotedValue = quoted + j.SaleLinesTotal
	if j.QuotedValue < 0 {
		j.QuotedValue = 0
	}
}

type RepairNote struct {
	ID         uuid.UUID  `json:"id"`
	Note       string     `json:"note"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	AuthorName *string    `json:"author_name,omitempty"`
}

type RepairAttachment struct {
	ID          uuid.UUID  `json:"id"`
	RepairJobID uuid.UUID  `json:"repair_job_id"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type"`
	SizeBytes   int        `json:"size_bytes"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
}

type CreateCustomerInput struct {
	FullName string
	Phone    *string
	Email    *string
	ActorID  uuid.UUID
	TenantID uuid.UUID
	CorrID   uuid.UUID
}

type CreateDeviceInput struct {
	CustomerID   *uuid.UUID
	Anonymous    bool
	Kind         string
	Brand        *string
	Model        *string
	IMEI         *string
	SerialNumber *string
	ActorID      uuid.UUID
	TenantID     uuid.UUID
	CorrID       uuid.UUID
}

type CreateRepairInput struct {
	BranchID       uuid.UUID
	CustomerID     *uuid.UUID
	DeviceID       uuid.UUID
	ProblemSummary string
	ServiceType    string
	TechnicianID   *uuid.UUID
	// LaborAmount is the price agreed with the customer at the counter. Leave it at
	// zero for the diagnose-first path, where the price is not known yet and comes
	// later as an estimate the customer approves.
	LaborAmount float64
	PromisedBy  *time.Time
	// CustomerWaiting means the customer is staying at the wait bench (not
	// dropping off and leaving). EstimatedWaitMinutes is the counter ETA.
	CustomerWaiting      bool
	EstimatedWaitMinutes *int
	CustomerCredit       bool
	CreditDueDate        *time.Time
	IntakeAccessories    []string
	IntakeCondition      *string
	DevicePasscode       string
	ParentJobID          *uuid.UUID
	ReworkReason         *string
	ActorID              uuid.UUID
	TenantID             uuid.UUID
	ClientID             *uuid.UUID
	CorrID               uuid.UUID
}

func (s *Service) CreateCustomer(ctx context.Context, in CreateCustomerInput) (*Customer, error) {
	var storePhone *string
	var variants []string
	if in.Phone != nil {
		raw := strings.TrimSpace(*in.Phone)
		if raw != "" {
			if e164, err := NormalizePhone(raw); err == nil {
				storePhone = &e164
				variants = PhoneMatchVariants(e164)
			} else {
				digits := digitsOnly(raw)
				if digits != "" {
					storePhone = &digits
					variants = PhoneMatchVariants(digits)
				}
			}
		}
	}

	if len(variants) > 0 {
		existing, err := s.findCustomerByPhoneVariants(ctx, in.TenantID, variants)
		if err == nil {
			// Keep stored phone in canonical E.164 when we know it.
			if storePhone != nil && (existing.Phone == nil || *existing.Phone != *storePhone) {
				_, _ = s.pool.Exec(ctx, `UPDATE repair.customers SET phone = $1, updated_at = now() WHERE id = $2`, *storePhone, existing.ID)
				existing.Phone = storePhone
			}
			return existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.customers (id, tenant_id, full_name, phone, email, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, in.TenantID, in.FullName, storePhone, in.Email, in.ActorID, in.CorrID)
	if err != nil {
		// Race: another request created the same phone — return that row.
		if len(variants) > 0 {
			if existing, qErr := s.findCustomerByPhoneVariants(ctx, in.TenantID, variants); qErr == nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return &Customer{ID: id, FullName: in.FullName, Phone: storePhone, Email: in.Email}, nil
}

type UpdateCustomerInput struct {
	CustomerID uuid.UUID
	FullName   string
	Phone      *string // nil or blank clears the phone
	Email      *string // nil or blank clears the email
	ActorID    uuid.UUID
	TenantID   uuid.UUID
}

func (s *Service) UpdateCustomer(ctx context.Context, in UpdateCustomerInput) (*Customer, error) {
	name := strings.TrimSpace(in.FullName)
	if name == "" {
		return nil, fmt.Errorf("full_name required")
	}

	var storePhone *string
	var variants []string
	if in.Phone != nil {
		raw := strings.TrimSpace(*in.Phone)
		if raw != "" {
			if e164, err := NormalizePhone(raw); err == nil {
				storePhone = &e164
				variants = PhoneMatchVariants(e164)
			} else {
				digits := digitsOnly(raw)
				if digits == "" {
					return nil, fmt.Errorf("invalid phone number")
				}
				storePhone = &digits
				variants = PhoneMatchVariants(digits)
			}
		}
	}

	if len(variants) > 0 {
		existing, err := s.findCustomerByPhoneVariants(ctx, in.TenantID, variants)
		if err == nil && existing.ID != in.CustomerID {
			return nil, fmt.Errorf("another customer already uses that phone number")
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	var storeEmail *string
	if in.Email != nil {
		raw := strings.TrimSpace(*in.Email)
		if raw != "" {
			storeEmail = &raw
		}
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE repair.customers
		SET full_name = $1, phone = $2, email = $3, updated_at = now()
		WHERE tenant_id = $4 AND id = $5`,
		name, storePhone, storeEmail, in.TenantID, in.CustomerID)
	if err != nil {
		if strings.Contains(err.Error(), "idx_customers_tenant_phone_normalized") ||
			strings.Contains(err.Error(), "idx_customers_tenant_email_unique") {
			return nil, fmt.Errorf("another customer already uses that phone or email")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("customer not found")
	}
	return &Customer{ID: in.CustomerID, FullName: name, Phone: storePhone, Email: storeEmail}, nil
}

// ClaimRepairJob attaches a repair to the customer identified by phone (creates if needed).
// It also backfills the device.customer_id so the customer app can see the job.
// ClaimRepairJob attaches a repair job to the authenticated customer's account.
// It only claims jobs that are unowned, or owned by a duplicate record with the
// same phone number — it never takes over a job linked to a different customer.
func (s *Service) ClaimRepairJob(ctx context.Context, tenantID, customerID uuid.UUID, jobCode string) (*RepairJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var claimantPhone *string
	err = tx.QueryRow(ctx, `
		SELECT phone FROM repair.customers WHERE tenant_id = $1 AND id = $2`,
		tenantID, customerID).Scan(&claimantPhone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("customer not found")
	}
	if err != nil {
		return nil, err
	}
	var variants []string
	if claimantPhone != nil {
		variants = PhoneMatchVariants(*claimantPhone)
	}

	var jobID, deviceID uuid.UUID
	var currentCustomerID *uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id, customer_id, device_id
		FROM repair.repair_jobs
		WHERE tenant_id = $1 AND UPPER(job_code) = UPPER($2)
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE`, tenantID, strings.TrimSpace(jobCode)).
		Scan(&jobID, &currentCustomerID, &deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("repair not found")
	}
	if err != nil {
		return nil, err
	}

	if currentCustomerID != nil && *currentCustomerID != customerID {
		// Allow the claim only when the current owner is a duplicate record
		// with the same phone number as the logged-in customer.
		var ownerDigits string
		err = tx.QueryRow(ctx, `
			SELECT regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g')
			FROM repair.customers WHERE tenant_id = $1 AND id = $2`,
			tenantID, *currentCustomerID).Scan(&ownerDigits)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		samePhone := false
		for _, v := range variants {
			if ownerDigits != "" && v == ownerDigits {
				samePhone = true
				break
			}
		}
		if !samePhone {
			return nil, fmt.Errorf("this repair is already linked to another customer; please contact the shop")
		}
	}

	if currentCustomerID == nil || *currentCustomerID != customerID {
		if _, err := tx.Exec(ctx, `UPDATE repair.repair_jobs SET customer_id = $1, updated_at = now() WHERE id = $2`, customerID, jobID); err != nil {
			return nil, err
		}
	}
	if deviceID != uuid.Nil {
		if _, err := tx.Exec(ctx, `UPDATE repair.devices SET customer_id = $1 WHERE id = $2`, customerID, deviceID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetRepair(ctx, tenantID, jobID)
}

func (s *Service) findCustomerByPhoneVariants(ctx context.Context, tenantID uuid.UUID, variants []string) (*Customer, error) {
	var existing Customer
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.full_name, c.phone, c.email FROM repair.customers c
		WHERE c.tenant_id = $1
		  AND regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
		ORDER BY
		  (SELECT COUNT(*) FROM repair.repair_jobs j WHERE j.customer_id = c.id) DESC,
		  c.created_at ASC
		LIMIT 1`, tenantID, variants).
		Scan(&existing.ID, &existing.FullName, &existing.Phone, &existing.Email)
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *Service) ListCustomers(ctx context.Context, tenantID uuid.UUID, q string, limit int) ([]Customer, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q = strings.TrimSpace(q)
	phoneDigits := digitsOnly(q)
	rows, err := s.pool.Query(ctx, `
		SELECT id, full_name, phone, email FROM repair.customers
		WHERE tenant_id = $1
		  AND (
		    $2 = ''
		    OR full_name ILIKE '%' || $2 || '%'
		    OR ($3 <> '' AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') LIKE '%' || $3 || '%')
		  )
		ORDER BY
		  CASE
		    WHEN $3 <> '' AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = $3 THEN 0
		    WHEN $3 <> '' AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') LIKE $3 || '%' THEN 1
		    WHEN full_name ILIKE $2 || '%' THEN 2
		    ELSE 3
		  END,
		  created_at DESC
		LIMIT $4`, tenantID, q, phoneDigits, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.FullName, &c.Phone, &c.Email); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}

func (s *Service) GetCustomer(ctx context.Context, tenantID, customerID uuid.UUID) (*Customer, []Device, []RepairJob, error) {
	var customer Customer
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, phone, email FROM repair.customers
		WHERE tenant_id = $1 AND id = $2`, tenantID, customerID).
		Scan(&customer.ID, &customer.FullName, &customer.Phone, &customer.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, fmt.Errorf("customer not found")
	}
	if err != nil {
		return nil, nil, nil, err
	}
	devices := make([]Device, 0)
	rows, err := s.pool.Query(ctx, `
		SELECT id, customer_id, anonymous, kind, brand, model, imei, serial_number
		FROM repair.devices WHERE tenant_id = $1 AND customer_id = $2 ORDER BY created_at DESC`,
		tenantID, customerID)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var device Device
		if err := rows.Scan(
			&device.ID, &device.CustomerID, &device.Anonymous, &device.Kind,
			&device.Brand, &device.Model, &device.IMEI, &device.SerialNumber,
		); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, nil, err
	}
	rows.Close()
	repairs, err := s.ListRepairs(ctx, tenantID, ListRepairsFilter{Search: customer.FullName})
	if err != nil {
		return nil, nil, nil, err
	}
	customerRepairs := make([]RepairJob, 0)
	for _, repair := range repairs {
		if repair.CustomerID != nil && *repair.CustomerID == customerID {
			customerRepairs = append(customerRepairs, repair)
		}
	}
	return &customer, devices, customerRepairs, nil
}

func (s *Service) PublicRepairStatus(ctx context.Context, jobCode, phone string) (*RepairJob, uuid.UUID, error) {
	variants := PhoneMatchVariants(phone)
	if len(variants) == 0 {
		return nil, uuid.Nil, fmt.Errorf("repair not found")
	}
	var tenantID, id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT j.tenant_id, j.id
		FROM repair.repair_jobs j
		JOIN repair.customers c ON c.id = j.customer_id
		WHERE UPPER(j.job_code) = UPPER($1)
		  AND regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
		ORDER BY j.created_at DESC LIMIT 1`, strings.TrimSpace(jobCode), variants).
		Scan(&tenantID, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.Nil, fmt.Errorf("repair not found")
	}
	if err != nil {
		return nil, uuid.Nil, err
	}
	job, err := s.GetRepair(ctx, tenantID, id)
	return job, tenantID, err
}

// PublicRepairStatusByCode looks up a repair by job code (or bare job number) for the default tenant.
func (s *Service) PublicRepairStatusByCode(ctx context.Context, jobCode string) (*RepairJob, uuid.UUID, error) {
	jobCode = strings.TrimSpace(jobCode)
	if jobCode == "" {
		return nil, uuid.Nil, fmt.Errorf("repair not found")
	}
	digits := digitsOnly(jobCode)
	upper := strings.ToUpper(jobCode)
	if strings.HasPrefix(upper, "JOB") && !strings.HasPrefix(upper, "JOB-") && digits != "" {
		upper = "JOB-" + digits
	}
	tenantID, err := s.DefaultTenantID(ctx)
	if err != nil {
		return nil, uuid.Nil, err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT j.id
		FROM repair.repair_jobs j
		WHERE j.tenant_id = $1
		  AND j.deleted_at IS NULL
		  AND (
		    UPPER(j.job_code) = $2
		    OR ($3 <> '' AND j.job_number::text = $3)
		  )
		ORDER BY j.created_at DESC LIMIT 1`, tenantID, upper, digits).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.Nil, fmt.Errorf("repair not found")
	}
	if err != nil {
		return nil, uuid.Nil, err
	}
	job, err := s.GetRepair(ctx, tenantID, id)
	return job, tenantID, err
}

type PublicReceipt struct {
	ID          uuid.UUID `json:"id"`
	Method      string    `json:"method"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	ProviderRef *string   `json:"provider_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Service) PublicRepairReceipts(ctx context.Context, tenantID, repairID uuid.UUID) ([]PublicReceipt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT p.id, p.method, p.amount::float8, p.currency, p.status,
			COALESCE(
				NULLIF(stk.mpesa_receipt, ''),
				NULLIF(c2b.trans_id, ''),
				p.provider_ref
			),
			p.created_at
		FROM payments.payments p
		JOIN payments.payment_allocations a ON a.payment_id = p.id
		LEFT JOIN LATERAL (
			SELECT mpesa_receipt FROM payments.mpesa_stk_transactions
			WHERE payment_id = p.id ORDER BY created_at DESC LIMIT 1
		) stk ON true
		LEFT JOIN LATERAL (
			SELECT trans_id FROM payments.mpesa_c2b_transactions
			WHERE payment_id = p.id AND status IS DISTINCT FROM 'superseded'
			ORDER BY created_at DESC LIMIT 1
		) c2b ON true
		WHERE p.tenant_id = $1 AND a.payable_type = 'repair' AND a.payable_id = $2
		  AND p.status NOT IN ('failed', 'cancelled')
		ORDER BY p.created_at DESC`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PublicReceipt, 0)
	for rows.Next() {
		var receipt PublicReceipt
		if err := rows.Scan(
			&receipt.ID, &receipt.Method, &receipt.Amount, &receipt.Currency,
			&receipt.Status, &receipt.ProviderRef, &receipt.CreatedAt,
		); err != nil {
			return nil, err
		}
		if receipt.ProviderRef != nil {
			if ref := receipts.CustomerPaymentRef(*receipt.ProviderRef); ref != "" {
				receipt.ProviderRef = &ref
			} else {
				receipt.ProviderRef = nil
			}
		}
		items = append(items, receipt)
	}
	return items, rows.Err()
}

func (s *Service) CreateDevice(ctx context.Context, in CreateDeviceInput) (*Device, error) {
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.devices (id, tenant_id, customer_id, anonymous, kind, brand, model, imei, serial_number, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id, in.TenantID, in.CustomerID, in.Anonymous, in.Kind, in.Brand, in.Model, in.IMEI, in.SerialNumber, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	return &Device{
		ID: id, CustomerID: in.CustomerID, Anonymous: in.Anonymous, Kind: in.Kind,
		Brand: in.Brand, Model: in.Model, IMEI: in.IMEI, SerialNumber: in.SerialNumber,
	}, nil
}

func FormatJobCode(jobNumber int) string {
	return fmt.Sprintf("JOB-%d", jobNumber)
}

func (s *Service) nextJobNumber(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (int, error) {
	var n int
	// First job for a tenant starts at 101; subsequent calls return the prior reserved number.
	err := tx.QueryRow(ctx, `
		INSERT INTO repair.job_counters (tenant_id, next_number)
		VALUES ($1, 102)
		ON CONFLICT (tenant_id) DO UPDATE
		SET next_number = repair.job_counters.next_number + 1
		RETURNING next_number - 1`, tenantID).Scan(&n)
	return n, err
}

func (s *Service) CreateRepair(ctx context.Context, in CreateRepairInput) (*RepairJob, error) {
	id := uuid.New()
	if in.ClientID != nil && *in.ClientID != uuid.Nil {
		id = *in.ClientID
	}
	status := StatusIntake
	serviceType := strings.TrimSpace(in.ServiceType)
	if serviceType == "" {
		serviceType = "repair"
	}
	if serviceType != "repair" && serviceType != "quick_replacement" && serviceType != "quick_fix" {
		return nil, fmt.Errorf("invalid service_type")
	}
	if in.CustomerCredit {
		if in.CustomerID == nil {
			return nil, fmt.Errorf("customer is required for credit")
		}
		if in.CreditDueDate == nil {
			return nil, fmt.Errorf("credit_due_date is required for credit")
		}
		due := in.CreditDueDate.UTC()
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if due.Before(today) {
			return nil, fmt.Errorf("credit_due_date cannot be in the past")
		}
		in.CreditDueDate = &due
	} else {
		in.CreditDueDate = nil
	}

	customerWaiting := in.CustomerWaiting
	var waitMinutes *int
	if customerWaiting {
		if in.EstimatedWaitMinutes == nil || *in.EstimatedWaitMinutes <= 0 {
			return nil, fmt.Errorf("estimated_wait_minutes is required when the customer is waiting")
		}
		mins := *in.EstimatedWaitMinutes
		if mins > 8*60 {
			return nil, fmt.Errorf("estimated_wait_minutes must be 480 or less")
		}
		waitMinutes = &mins
		// If staff did not pick an absolute promise time, derive one from the wait.
		if in.PromisedBy == nil {
			ready := time.Now().UTC().Add(time.Duration(mins) * time.Minute)
			in.PromisedBy = &ready
		}
	} else if in.EstimatedWaitMinutes != nil && *in.EstimatedWaitMinutes > 0 {
		// Ignore stray wait minutes when the customer is leaving the device.
		waitMinutes = nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if in.CorrID != uuid.Nil {
		var existing RepairJob
		err = tx.QueryRow(ctx, `
			SELECT id, job_number, job_code, branch_id, customer_id, device_id, technician_id, status, problem_summary
			FROM repair.repair_jobs WHERE tenant_id = $1 AND correlation_id = $2`,
			in.TenantID, in.CorrID).Scan(
			&existing.ID, &existing.JobNumber, &existing.JobCode, &existing.BranchID,
			&existing.CustomerID, &existing.DeviceID, &existing.TechnicianID, &existing.Status, &existing.ProblemSummary,
		)
		if err == nil {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return &existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
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

	labor := in.LaborAmount
	if labor < 0 {
		labor = 0
	}
	// A price entered at intake is a price the customer standing at the counter has
	// agreed to, so it doubles as the authorisation to start work. Leaving it blank
	// is the diagnose-first path: the job waits for an approved estimate.
	var authAt *time.Time
	var authSource *string
	var authAmount *float64
	if labor > 0 {
		now := time.Now().UTC()
		src := AuthSourceIntakeAgreed
		authAt, authSource, authAmount = &now, &src, &labor
	}
	accessoriesJSON, condition, err := intakePayload(in.IntakeAccessories, in.IntakeCondition)
	if err != nil {
		return nil, err
	}
	passcodeCiphertext, err := s.encryptPasscode(in.DevicePasscode)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_jobs (
			id, tenant_id, branch_id, customer_id, device_id, technician_id, status, problem_summary,
			labor_amount, job_number, job_code, pickup_code, created_by, updated_by, correlation_id,
			work_authorized_at, work_authorization_source, authorized_amount, work_authorized_by,
			promised_by, intake_accessories, intake_condition, device_passcode_ciphertext,
			parent_job_id, rework_reason, customer_waiting, estimated_wait_minutes, customer_credit, credit_due_date, service_type
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13, $14, $15, $16, $17,
		        CASE WHEN $15::timestamptz IS NULL THEN NULL ELSE $13::uuid END,
		        $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28)
		ON CONFLICT (id) DO NOTHING`,
		id, in.TenantID, in.BranchID, in.CustomerID, in.DeviceID, in.TechnicianID, status, in.ProblemSummary,
		labor, jobNumber, jobCode, pickupCode, in.ActorID, in.CorrID, authAt, authSource, authAmount, in.PromisedBy,
		accessoriesJSON, condition, passcodeCiphertext, in.ParentJobID, in.ReworkReason, customerWaiting, waitMinutes, in.CustomerCredit, in.CreditDueDate, serviceType)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var replay RepairJob
		err = tx.QueryRow(ctx, `
			SELECT id, job_number, job_code, branch_id, customer_id, device_id, technician_id, status, problem_summary
			FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, in.TenantID, id).Scan(
			&replay.ID, &replay.JobNumber, &replay.JobCode, &replay.BranchID,
			&replay.CustomerID, &replay.DeviceID, &replay.TechnicianID, &replay.Status, &replay.ProblemSummary,
		)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &replay, nil
	}
	evID := uuid.New()
	intakeNote := "Checked in — diagnose first, price to be quoted"
	if labor > 0 {
		intakeNote = fmt.Sprintf("Checked in — price agreed at the counter: %.2f", labor)
	}
	if customerWaiting && waitMinutes != nil {
		intakeNote += fmt.Sprintf(" — customer waiting at the bench (~%d min)", *waitMinutes)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, evID, in.TenantID, id, status, intakeNote, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	job := &RepairJob{
		ID: id, JobNumber: jobNumber, JobCode: jobCode, PickupCode: pickupCode,
		BranchID: in.BranchID, CustomerID: in.CustomerID, DeviceID: in.DeviceID,
		TechnicianID: in.TechnicianID, Status: status, ProblemSummary: in.ProblemSummary, ServiceType: serviceType,
		LaborAmount:       labor,
		PromisedBy:        in.PromisedBy,
		CustomerWaiting:   customerWaiting,
		EstimatedWaitMin:  waitMinutes,
		CustomerCredit:    in.CustomerCredit,
		CreditDueDate:     in.CreditDueDate,
		IntakeAccessories: normalizeAccessories(in.IntakeAccessories),
		IntakeCondition:   condition, HasDevicePasscode: len(passcodeCiphertext) > 0,
		ParentJobID: in.ParentJobID, ReworkReason: in.ReworkReason,
	}
	payload := map[string]any{
		"repair_job_id": id.String(), "job_code": jobCode, "pickup_code": pickupCode, "status": status,
		"labor_amount": labor, "problem_summary": in.ProblemSummary,
		"price_agreed_at_intake": labor > 0,
		"customer_waiting":       customerWaiting,
	}
	if waitMinutes != nil {
		payload["estimated_wait_minutes"] = *waitMinutes
	}
	s.publish("repair.created", in.TenantID, in.BranchID, in.ActorID, in.CorrID, payload)
	return job, nil
}

type ListRepairsFilter struct {
	BranchID     *uuid.UUID
	Status       string
	TechnicianID *uuid.UUID
	Search       string
}

func (s *Service) ListRepairs(ctx context.Context, tenantID uuid.UUID, f ListRepairsFilter) ([]RepairJob, error) {
	q := `SELECT j.id, j.job_number, j.job_code, j.branch_id, j.customer_id, c.full_name, j.device_id, j.technician_id, j.status, j.problem_summary, j.service_type,
			COALESCE(j.labor_amount, 0)::float8, j.created_at, j.promised_by, j.customer_waiting, j.estimated_wait_minutes, j.customer_credit, j.credit_due_date,
			j.parent_job_id, parent.job_code, j.rework_reason,
			j.authorized_amount::float8,
			COALESCE((
				SELECT SUM(sl.line_total)::float8 FROM repair.job_sale_lines sl
				WHERE sl.tenant_id = j.tenant_id AND sl.repair_job_id = j.id
			), 0),
			(
				SELECT (e.labor_amount + e.parts_amount)::float8
				FROM repair.repair_estimates e
				WHERE e.tenant_id = j.tenant_id AND e.repair_job_id = j.id AND e.status = 'approved'
				ORDER BY e.decided_at DESC NULLS LAST, e.created_at DESC
				LIMIT 1
			),
			(
				SELECT (e.labor_amount + e.parts_amount)::float8
				FROM repair.repair_estimates e
				WHERE e.tenant_id = j.tenant_id AND e.repair_job_id = j.id AND e.status = 'pending'
				ORDER BY e.created_at DESC
				LIMIT 1
			),
			COALESCE((
				SELECT SUM(a.amount)::float8
				FROM payments.payment_allocations a
				JOIN payments.payments p ON p.id = a.payment_id
				WHERE a.tenant_id = j.tenant_id
				  AND a.payable_type = 'repair'
				  AND a.payable_id = j.id
				  AND p.status IN ('allocated', 'confirmed', 'provisional')
			), 0),
			d.kind, d.brand, d.model, d.imei
		FROM repair.repair_jobs j
		LEFT JOIN repair.customers c ON c.id = j.customer_id
		LEFT JOIN repair.devices d ON d.id = j.device_id
		LEFT JOIN repair.repair_jobs parent ON parent.id = j.parent_job_id
		WHERE j.tenant_id = $1 AND j.deleted_at IS NULL`
	args := []any{tenantID}
	n := 2
	if f.BranchID != nil {
		q += fmt.Sprintf(" AND j.branch_id = $%d", n)
		args = append(args, *f.BranchID)
		n++
	}
	switch f.Status {
	case "":
		// no status filter
	case "open":
		// Pseudo-filter for the board: still on the bench, not handed back and not written off.
		q += " AND j.status NOT IN ('collected', 'cancelled', 'unrepairable')"
	case "ready":
		// Shop-floor "Ready": QC done or marked complete, not yet collected.
		q += " AND j.status IN ('ready_for_pickup', 'completed')"
	case "closed":
		q += " AND j.status IN ('cancelled', 'unrepairable')"
	case "overdue":
		q += " AND j.promised_by < now() AND j.status NOT IN ('ready_for_pickup', 'completed', 'collected', 'cancelled', 'unrepairable')"
	default:
		q += fmt.Sprintf(" AND j.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.TechnicianID != nil {
		q += fmt.Sprintf(" AND j.technician_id = $%d", n)
		args = append(args, *f.TechnicianID)
		n++
	}
	if f.Search != "" {
		q += fmt.Sprintf(` AND (
			j.job_code ILIKE '%%' || $%d || '%%'
			OR j.problem_summary ILIKE '%%' || $%d || '%%'
			OR c.full_name ILIKE '%%' || $%d || '%%'
			OR COALESCE(j.pickup_code, '') ILIKE '%%' || $%d || '%%'
			OR COALESCE(d.brand, '') ILIKE '%%' || $%d || '%%'
			OR COALESCE(d.model, '') ILIKE '%%' || $%d || '%%'
			OR COALESCE(d.imei, '') ILIKE '%%' || $%d || '%%'
		)`, n, n, n, n, n, n, n)
		args = append(args, f.Search)
		n++
	}
	// Shop-floor order: overdue first, wait-bench next, then workflow stage,
	// then soonest promise, then oldest intake (FIFO).
	q += ` ORDER BY
		CASE
			WHEN j.promised_by IS NOT NULL
				AND j.promised_by < now()
				AND j.status NOT IN ('ready_for_pickup', 'completed', 'collected', 'cancelled', 'unrepairable')
			THEN 0 ELSE 1
		END,
		CASE WHEN j.customer_waiting AND j.status NOT IN ('collected', 'cancelled', 'unrepairable') THEN 0 ELSE 1 END,
		CASE j.status
			WHEN 'waiting_parts' THEN 1
			WHEN 'in_progress' THEN 2
			WHEN 'diagnosed' THEN 3
			WHEN 'intake' THEN 4
			WHEN 'ready_for_pickup' THEN 5
			WHEN 'completed' THEN 6
			WHEN 'collected' THEN 7
			WHEN 'cancelled' THEN 8
			WHEN 'unrepairable' THEN 9
			ELSE 5
		END,
		j.promised_by ASC NULLS LAST,
		j.created_at ASC
		LIMIT 100`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RepairJob
	for rows.Next() {
		var j RepairJob
		var device Device
		var deviceKind *string
		if err := rows.Scan(
			&j.ID, &j.JobNumber, &j.JobCode, &j.BranchID, &j.CustomerID, &j.CustomerName, &j.DeviceID, &j.TechnicianID,
			&j.Status, &j.ProblemSummary, &j.ServiceType, &j.LaborAmount, &j.CreatedAt, &j.PromisedBy, &j.CustomerWaiting, &j.EstimatedWaitMin, &j.CustomerCredit, &j.CreditDueDate,
			&j.ParentJobID, &j.ParentJobCode, &j.ReworkReason,
			&j.AuthorizedAmount, &j.SaleLinesTotal, &j.ApprovedEstimateTotal, &j.PendingEstimateTotal, &j.PaidTotal,
			&deviceKind, &device.Brand, &device.Model, &device.IMEI,
		); err != nil {
			return nil, err
		}
		device.ID = j.DeviceID
		if deviceKind != nil {
			device.Kind = *deviceKind
		}
		if device.Brand != nil || device.Model != nil || device.IMEI != nil || device.Kind != "" {
			j.Device = &device
		}
		applyJobMoney(&j)
		if j.AuthorizedAmount != nil || j.Authorization != nil {
			if j.Authorization == nil {
				j.Authorization = &WorkAuthorization{}
			}
			j.Authorization.AuthorizedAmount = j.AuthorizedAmount
		}
		items = append(items, j)
	}
	return items, nil
}

func (s *Service) GetRepair(ctx context.Context, tenantID, id uuid.UUID) (*RepairJob, error) {
	var j RepairJob
	var auth WorkAuthorization
	var authSource *string
	var accessoriesJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT j.id, j.job_number, j.job_code, COALESCE(j.pickup_code, ''), j.branch_id, j.customer_id, j.device_id, j.technician_id, j.status, j.problem_summary, j.service_type,
		       COALESCE(j.labor_amount, 0)::float8, j.created_at, j.closure_reason, j.closed_at, j.version,
		       j.work_authorized_at, j.work_authorized_by, j.work_authorization_source,
		       j.authorized_amount::float8, j.labor_variance_reason, j.promised_by,
		       j.customer_waiting, j.estimated_wait_minutes, j.customer_credit, j.credit_due_date,
		       j.intake_accessories, j.intake_condition,
		       (j.device_passcode_ciphertext IS NOT NULL), j.parent_job_id, parent.job_code, j.rework_reason
		FROM repair.repair_jobs j
		LEFT JOIN repair.repair_jobs parent ON parent.id = j.parent_job_id
		WHERE j.tenant_id = $1 AND j.id = $2 AND j.deleted_at IS NULL`, tenantID, id).
		Scan(&j.ID, &j.JobNumber, &j.JobCode, &j.PickupCode, &j.BranchID, &j.CustomerID, &j.DeviceID, &j.TechnicianID, &j.Status, &j.ProblemSummary, &j.ServiceType,
			&j.LaborAmount, &j.CreatedAt, &j.ClosureReason, &j.ClosedAt, &j.Version,
			&auth.AuthorizedAt, &auth.AuthorizedBy, &authSource, &auth.AuthorizedAmount, &auth.VarianceReason, &j.PromisedBy,
			&j.CustomerWaiting, &j.EstimatedWaitMin, &j.CustomerCredit, &j.CreditDueDate,
			&accessoriesJSON, &j.IntakeCondition, &j.HasDevicePasscode, &j.ParentJobID, &j.ParentJobCode, &j.ReworkReason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	if authSource != nil {
		auth.Source = *authSource
	}
	j.Authorization = &auth
	_ = json.Unmarshal(accessoriesJSON, &j.IntakeAccessories)
	if j.CustomerID != nil {
		var c Customer
		if err := s.pool.QueryRow(ctx, `
			SELECT id, full_name, phone, email FROM repair.customers WHERE tenant_id = $1 AND id = $2`,
			tenantID, *j.CustomerID).Scan(&c.ID, &c.FullName, &c.Phone, &c.Email); err == nil {
			j.Customer = &c
			j.CustomerName = &c.FullName
		}
	}
	var d Device
	if err := s.pool.QueryRow(ctx, `
		SELECT id, customer_id, anonymous, kind, brand, model, imei, serial_number
		FROM repair.devices WHERE tenant_id = $1 AND id = $2`,
		tenantID, j.DeviceID).Scan(&d.ID, &d.CustomerID, &d.Anonymous, &d.Kind, &d.Brand, &d.Model, &d.IMEI, &d.SerialNumber); err == nil {
		j.Device = &d
	}
	j.Timeline, err = s.loadTimeline(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(j.PickupCode) == "" && j.Status != StatusCollected {
		if code, err := s.EnsurePickupCode(ctx, tenantID, id); err == nil {
			j.PickupCode = code
		}
	}
	if lines, err := s.ListJobSaleLines(ctx, tenantID, id); err == nil {
		j.SaleLines = lines
		for _, line := range lines {
			j.SaleLinesTotal += line.LineTotal
		}
	}
	// Same money rollups as ListRepairs so detail screens and handover share one formula.
	var approvedTotal, pendingTotal float64
	if err := s.pool.QueryRow(ctx, `
		SELECT (labor_amount + parts_amount)::float8
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status = 'approved'
		ORDER BY decided_at DESC NULLS LAST, created_at DESC LIMIT 1`, tenantID, id).
		Scan(&approvedTotal); err == nil {
		j.ApprovedEstimateTotal = &approvedTotal
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT (labor_amount + parts_amount)::float8
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1`, tenantID, id).
		Scan(&pendingTotal); err == nil {
		j.PendingEstimateTotal = &pendingTotal
	}
	j.AuthorizedAmount = auth.AuthorizedAmount
	if _, balance, total, _, _, err := s.repairPaymentAmounts(ctx, tenantID, id); err == nil {
		j.PaidTotal = total - balance
		if j.PaidTotal < 0 {
			j.PaidTotal = 0
		}
	}
	applyJobMoney(&j)
	return &j, nil
}

// UpdatePromisedBy changes the customer commitment without pretending the job
// moved stage. Clearing the value is supported when no date was promised.
func (s *Service) UpdatePromisedBy(ctx context.Context, tenantID, repairID uuid.UUID, promisedBy *time.Time, actorID, corrID uuid.UUID) (*RepairJob, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET promised_by = $1, updated_by = $2, updated_at = now()
		WHERE tenant_id = $3 AND id = $4`,
		promisedBy, actorID, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("repair not found")
	}
	note := "Promised date cleared"
	if promisedBy != nil {
		note = "Promised to customer by " + promisedBy.UTC().Format(time.RFC3339)
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events
			(id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		SELECT $1, tenant_id, id, status, $2, $3, $4
		FROM repair.repair_jobs WHERE tenant_id = $5 AND id = $6`,
		uuid.New(), note, actorID, corrID, tenantID, repairID)
	return s.GetRepair(ctx, tenantID, repairID)
}

func (s *Service) AddNote(ctx context.Context, tenantID, repairID uuid.UUID, note string, actorID, corrID uuid.UUID, clientID *uuid.UUID) (*RepairNote, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2)`,
		tenantID, repairID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("repair not found")
	}
	if corrID != uuid.Nil {
		var replay RepairNote
		err := s.pool.QueryRow(ctx, `
			SELECT id, note, created_at, created_by FROM repair.repair_notes
			WHERE tenant_id = $1 AND correlation_id = $2`, tenantID, corrID).
			Scan(&replay.ID, &replay.Note, &replay.CreatedAt, &replay.CreatedBy)
		if err == nil {
			return &replay, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	id := uuid.New()
	if clientID != nil && *clientID != uuid.Nil {
		id = *clientID
	}
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO repair.repair_notes (id, tenant_id, repair_job_id, note, created_at, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING`,
		id, tenantID, repairID, note, now, actorID, corrID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var replay RepairNote
		err = s.pool.QueryRow(ctx, `
			SELECT id, note, created_at, created_by FROM repair.repair_notes
			WHERE tenant_id = $1 AND id = $2`, tenantID, id).
			Scan(&replay.ID, &replay.Note, &replay.CreatedAt, &replay.CreatedBy)
		if err != nil {
			return nil, err
		}
		return &replay, nil
	}
	s.AdvanceStatusIf(ctx, tenantID, repairID,
		[]string{StatusIntake}, StatusDiagnosed,
		"Diagnosis note added", actorID, corrID)
	return &RepairNote{ID: id, Note: note, CreatedAt: now, CreatedBy: &actorID}, nil
}

func (s *Service) ListNotes(ctx context.Context, tenantID, repairID uuid.UUID) ([]RepairNote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.note, n.created_at, n.created_by, u.display_name
		FROM repair.repair_notes n
		LEFT JOIN identity.users u ON u.id = n.created_by
		WHERE n.tenant_id = $1 AND n.repair_job_id = $2
		ORDER BY n.created_at ASC`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RepairNote
	for rows.Next() {
		var n RepairNote
		if err := rows.Scan(&n.ID, &n.Note, &n.CreatedAt, &n.CreatedBy, &n.AuthorName); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, nil
}

func (s *Service) AddAttachment(
	ctx context.Context,
	tenantID, repairID uuid.UUID,
	fileName, contentType string,
	content []byte,
	actorID, corrID uuid.UUID,
	clientID *uuid.UUID,
) (*RepairAttachment, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2)`,
		tenantID, repairID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("repair not found")
	}
	if corrID != uuid.Nil {
		var replay RepairAttachment
		err := s.pool.QueryRow(ctx, `
			SELECT id, repair_job_id, file_name, content_type, size_bytes, created_at, created_by
			FROM repair.repair_attachments
			WHERE tenant_id = $1 AND correlation_id = $2`, tenantID, corrID).Scan(
			&replay.ID, &replay.RepairJobID, &replay.FileName, &replay.ContentType,
			&replay.SizeBytes, &replay.CreatedAt, &replay.CreatedBy,
		)
		if err == nil {
			return &replay, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	id := uuid.New()
	if clientID != nil && *clientID != uuid.Nil {
		id = *clientID
	}
	now := time.Now().UTC()
	var storageKey *string
	var dbContent []byte
	if s.store != nil {
		key := objectstore.AttachmentKey(tenantID.String(), repairID.String(), id.String(), fileName)
		if err := s.store.Put(ctx, key, content, contentType); err != nil {
			return nil, fmt.Errorf("object storage upload: %w", err)
		}
		storageKey = &key
	} else {
		dbContent = content
	}
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO repair.repair_attachments
			(id, tenant_id, repair_job_id, file_name, content_type, content, size_bytes,
			 storage_key, upload_status, created_at, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO NOTHING`,
		id, tenantID, repairID, fileName, contentType, dbContent, len(content),
		storageKey, AttachmentUploadCompleted, now, actorID, corrID)
	if err != nil {
		if storageKey != nil && s.store != nil {
			_ = s.store.Delete(ctx, *storageKey)
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var replay RepairAttachment
		err = s.pool.QueryRow(ctx, `
			SELECT id, repair_job_id, file_name, content_type, size_bytes, created_at, created_by
			FROM repair.repair_attachments
			WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(
			&replay.ID, &replay.RepairJobID, &replay.FileName, &replay.ContentType,
			&replay.SizeBytes, &replay.CreatedAt, &replay.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		return &replay, nil
	}
	return &RepairAttachment{
		ID: id, RepairJobID: repairID, FileName: fileName, ContentType: contentType,
		SizeBytes: len(content), CreatedAt: now, CreatedBy: &actorID,
	}, nil
}

func (s *Service) ListAttachments(ctx context.Context, tenantID, repairID uuid.UUID) ([]RepairAttachment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repair_job_id, file_name, content_type, size_bytes, created_at, created_by
		FROM repair.repair_attachments
		WHERE tenant_id = $1 AND repair_job_id = $2 AND upload_status = $3
		ORDER BY created_at DESC`, tenantID, repairID, AttachmentUploadCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RepairAttachment, 0)
	for rows.Next() {
		var item RepairAttachment
		if err := rows.Scan(
			&item.ID, &item.RepairJobID, &item.FileName, &item.ContentType,
			&item.SizeBytes, &item.CreatedAt, &item.CreatedBy,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetAttachmentContent(ctx context.Context, tenantID, repairID, attachmentID uuid.UUID) (string, string, []byte, error) {
	var fileName, contentType, uploadStatus string
	var content []byte
	var storageKey *string
	err := s.pool.QueryRow(ctx, `
		SELECT file_name, content_type, content, storage_key, upload_status
		FROM repair.repair_attachments
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, attachmentID).Scan(&fileName, &contentType, &content, &storageKey, &uploadStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, fmt.Errorf("attachment not found")
	}
	if err != nil {
		return "", "", nil, err
	}
	if uploadStatus == AttachmentUploadPending {
		return "", "", nil, fmt.Errorf("attachment upload is not complete")
	}
	if storageKey != nil && *storageKey != "" {
		if s.store == nil {
			return "", "", nil, fmt.Errorf("attachment is in object storage but OBJECT_STORAGE_* is not configured")
		}
		body, gerr := s.store.Get(ctx, *storageKey)
		if gerr != nil {
			return "", "", nil, fmt.Errorf("object storage download: %w", gerr)
		}
		return fileName, contentType, body, nil
	}
	return fileName, contentType, content, nil
}

func (s *Service) DeleteAttachment(ctx context.Context, tenantID, repairID, attachmentID uuid.UUID) error {
	var storageKey *string
	_ = s.pool.QueryRow(ctx, `
		SELECT storage_key FROM repair.repair_attachments
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, attachmentID).Scan(&storageKey)
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM repair.repair_attachments
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, attachmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("attachment not found")
	}
	if storageKey != nil && *storageKey != "" && s.store != nil {
		_ = s.store.Delete(ctx, *storageKey)
	}
	return nil
}

func (s *Service) Assign(ctx context.Context, tenantID, repairID, technicianID, actorID, corrID uuid.UUID) (*RepairJob, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE repair.repair_jobs SET technician_id = $1, updated_by = $2, updated_at = now(), version = version + 1
		WHERE tenant_id = $3 AND id = $4`, technicianID, actorID, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("repair not found")
	}
	s.publish("repair.assigned", tenantID, uuid.Nil, actorID, corrID, map[string]any{
		"repair_job_id": repairID.String(), "technician_id": technicianID.String(),
	})
	// Claiming a job must not bypass work authorisation. AdvanceStatusIf enforces
	// the same intake/diagnosed → in_progress gate as ChangeStatus.
	s.AdvanceStatusIf(ctx, tenantID, repairID,
		[]string{StatusIntake}, StatusInProgress,
		"Claimed by technician", actorID, corrID)
	return s.GetRepair(ctx, tenantID, repairID)
}

// ErrBalanceDue is returned when a caller tries to mark a repair collected while money is still owed.
var ErrBalanceDue = errors.New("balance due — collect payment before marking the device collected")

// ErrInvalidClosure is returned when a job is closed without a valid reason code.
var ErrInvalidClosure = errors.New("invalid closure")

// ChangeStatusInput describes a single status transition on a repair job.
type ChangeStatusInput struct {
	TenantID      uuid.UUID
	RepairID      uuid.UUID
	NewStatus     string
	Note          *string
	LaborAmount   *float64
	ClosureReason string
	// VarianceReason explains a final charge above what the customer agreed to.
	VarianceReason string
	ActorID        uuid.UUID
	CorrelationID  uuid.UUID
	Force          bool
}

func (s *Service) ChangeStatus(ctx context.Context, in ChangeStatusInput) (*RepairJob, error) {
	tenantID, repairID, newStatus := in.TenantID, in.RepairID, in.NewStatus
	actorID, corrID := in.ActorID, in.CorrelationID
	var current string
	var branchID uuid.UUID
	var technicianID *uuid.UUID
	var existingLabor float64
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id, technician_id, COALESCE(labor_amount, 0)::float8
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, repairID).
		Scan(&current, &branchID, &technicianID, &existingLabor)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	if err := ValidateStatusTransition(current, newStatus); err != nil {
		return nil, err
	}
	closing := IsClosure(newStatus)
	if closing {
		if err := ValidateClosureReason(newStatus, in.ClosureReason); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidClosure, err.Error())
		}
	}
	// Collected means the customer physically has the device, so it is reached by
	// recording the handover — who took it and how we verified them — not by
	// picking a status off a list. Handover does its own balance check.
	if newStatus == StatusCollected {
		return nil, ErrHandoverRequired
	}

	auth, err := s.loadAuthorization(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	// The gate applies only on first entry onto the bench. Coming back from
	// waiting_parts means work was already authorized before the parts detour.
	if newStatus == StatusInProgress && (current == StatusIntake || current == StatusDiagnosed) && auth.AuthorizedAt == nil {
		return nil, ErrWorkNotAuthorized
	}

	// Do not mark ready / complete (or skip waiting_parts) while supplier parts
	// or a customer estimate are still unresolved.
	if RequiresClearPartsAndEstimates(newStatus) || LeavingWaitingPartsForBench(current, newStatus) {
		if n, err := s.countOutstandingParts(ctx, tenantID, repairID); err != nil {
			return nil, err
		} else if n > 0 {
			return nil, fmt.Errorf("%w (%d open)", ErrPartsOutstanding, n)
		}
	}
	if RequiresClearPartsAndEstimates(newStatus) {
		if pending, err := s.hasPendingEstimate(ctx, tenantID, repairID); err != nil {
			return nil, err
		} else if pending {
			return nil, ErrEstimatePending
		}
	}

	labor := existingLabor
	if in.LaborAmount != nil {
		labor = *in.LaborAmount
	} else if closing {
		// No work was delivered, so any price quoted at intake must not survive
		// as a phantom balance — it would otherwise block handing the device back.
		// A diagnostic fee is still chargeable by passing LaborAmount explicitly.
		labor = 0
	}

	// Charging more than the customer agreed to is allowed, but never silently:
	// the overrun has to be explained, and the explanation lands on the timeline.
	varianceReason := strings.TrimSpace(in.VarianceReason)
	overrun := newStatus == StatusCompleted && exceedsAuthorizedAmount(auth.AuthorizedAmount, labor)
	if overrun && varianceReason == "" {
		return nil, fmt.Errorf("%w (authorized %.2f, final %.2f)", ErrVarianceReasonRequired, *auth.AuthorizedAmount, labor)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if closing {
		_, err = tx.Exec(ctx, `
			UPDATE repair.repair_jobs
			SET status = $1, labor_amount = $2, closure_reason = $3, closed_at = now(),
			    updated_by = $4, updated_at = now(), version = version + 1
			WHERE tenant_id = $5 AND id = $6`,
			newStatus, labor, strings.TrimSpace(in.ClosureReason), actorID, tenantID, repairID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE repair.repair_jobs SET status = $1, labor_amount = $2, updated_by = $3, updated_at = now(), version = version + 1
			WHERE tenant_id = $4 AND id = $5`, newStatus, labor, actorID, tenantID, repairID)
	}
	if err != nil {
		return nil, err
	}
	if overrun {
		if _, err := tx.Exec(ctx, `
			UPDATE repair.repair_jobs SET labor_variance_reason = $1 WHERE tenant_id = $2 AND id = $3`,
			varianceReason, tenantID, repairID); err != nil {
			return nil, err
		}
	}
	evID := uuid.New()
	note := in.Note
	if overrun {
		overrunNote := fmt.Sprintf("Final charge %.2f exceeds authorized %.2f — %s",
			labor, *auth.AuthorizedAmount, varianceReason)
		if note != nil && strings.TrimSpace(*note) != "" {
			overrunNote = strings.TrimSpace(*note) + " · " + overrunNote
		}
		note = &overrunNote
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, evID, tenantID, repairID, newStatus, note, actorID, corrID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.publish("repair.status_changed", tenantID, branchID, actorID, corrID, map[string]any{
		"repair_job_id": repairID.String(), "from": current, "to": newStatus,
	})
	if closing {
		s.publish("repair.closed", tenantID, branchID, actorID, corrID, map[string]any{
			"repair_job_id": repairID.String(), "status": newStatus, "reason": strings.TrimSpace(in.ClosureReason),
		})
	}
	if overrun {
		s.publish("repair.labor_variance", tenantID, branchID, actorID, corrID, map[string]any{
			"repair_job_id":     repairID.String(),
			"authorized_amount": *auth.AuthorizedAmount,
			"final_amount":      labor,
			"variance":          LaborVariance(auth.AuthorizedAmount, labor),
			"reason":            varianceReason,
		})
	}
	if s.completionHook != nil {
		_ = s.completionHook.OnRepairStatusChanged(ctx, tenantID, repairID, newStatus, actorID)
	}
	if newStatus == StatusCompleted {
		s.fireRepairCompletedSideEffects(ctx, tenantID, branchID, repairID, actorID, corrID)
	}
	s.maybeEnsureWarranty(ctx, tenantID, repairID, newStatus)
	return s.GetRepair(ctx, tenantID, repairID)
}

func (s *Service) countOutstandingParts(ctx context.Context, tenantID, repairID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM inventory.part_requests
		WHERE tenant_id = $1 AND repair_job_id = $2
		  AND status IN ('pending', 'approved')`, tenantID, repairID).Scan(&n)
	return n, err
}

func (s *Service) hasPendingEstimate(ctx context.Context, tenantID, repairID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM repair.repair_estimates
			WHERE tenant_id = $1 AND repair_job_id = $2
			  AND status = 'pending'
			  AND (expires_at IS NULL OR expires_at > now())
		)`, tenantID, repairID).Scan(&exists)
	return exists, err
}

// AdvanceStatusIf moves a repair forward when from matches current status.
// Best-effort: failures are swallowed so callers (notes, parts, payments) never fail.
func (s *Service) AdvanceStatusIf(ctx context.Context, tenantID, repairID uuid.UUID, from []string, to, note string, actorID, corrID uuid.UUID) {
	if repairID == uuid.Nil || to == "" {
		return
	}
	var current string
	var branchID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&current, &branchID)
	if err != nil {
		return
	}
	matched := false
	for _, f := range from {
		if f == current {
			matched = true
			break
		}
	}
	if !matched {
		return
	}
	if err := ValidateStatusTransition(current, to); err != nil {
		return
	}
	// Same gate as ChangeStatus: do not start bench work without an agreed price /
	// manager go-ahead. Assign used to skip this and silently open unauthorized jobs.
	if to == StatusInProgress && (current == StatusIntake || current == StatusDiagnosed) {
		auth, err := s.loadAuthorization(ctx, tenantID, repairID)
		if err != nil || auth.AuthorizedAt == nil {
			return
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs SET status = $1, updated_by = $2, updated_at = now(), version = version + 1
		WHERE tenant_id = $3 AND id = $4 AND status = $5`,
		to, actorID, tenantID, repairID, current)
	if err != nil || tag.RowsAffected() == 0 {
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairID, to, note, actorID, corrID); err != nil {
		return
	}
	if err := tx.Commit(ctx); err != nil {
		return
	}
	s.publish("repair.status_changed", tenantID, branchID, actorID, corrID, map[string]any{
		"repair_job_id": repairID.String(), "from": current, "to": to,
	})
	if s.completionHook != nil {
		_ = s.completionHook.OnRepairStatusChanged(ctx, tenantID, repairID, to, actorID)
	}
	if to == StatusCompleted {
		s.fireRepairCompletedSideEffects(ctx, tenantID, branchID, repairID, actorID, corrID)
	}
	s.maybeEnsureWarranty(ctx, tenantID, repairID, to)
}

// fireRepairCompletedSideEffects runs loyalty/commission/notify hooks once a job
// reaches completed — including when collection jumps ready_for_pickup → collected.
func (s *Service) fireRepairCompletedSideEffects(ctx context.Context, tenantID, branchID, repairID, actorID, corrID uuid.UUID) {
	var technicianID *uuid.UUID
	var labor float64
	_ = s.pool.QueryRow(ctx, `
		SELECT technician_id, COALESCE(labor_amount, 0)::float8
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&technicianID, &labor)

	s.publish("repair.completed", tenantID, branchID, actorID, corrID, map[string]any{
		"repair_job_id": repairID.String(), "labor_amount": labor,
	})
	if s.commissionHook != nil && technicianID != nil {
		if err := s.commissionHook.AccrueOnRepairCompleted(ctx, tenantID, branchID, repairID, *technicianID, labor, actorID, corrID); err != nil {
			s.publish("commission.accrual_failed", tenantID, branchID, actorID, corrID, map[string]any{
				"repair_job_id": repairID.String(), "error": err.Error(),
			})
		} else {
			s.publish("commission.accrued", tenantID, branchID, actorID, corrID, map[string]any{
				"repair_job_id": repairID.String(), "technician_id": technicianID.String(),
			})
		}
	}
	if s.completionHook != nil {
		_ = s.completionHook.OnRepairCompleted(ctx, tenantID, repairID, actorID)
	}
}

// outstandingRepairBalance returns the amount still owed on a repair.
// Paid amounts (including any legacy provisional rows) are already credited inside
// RepairPaymentContext — do not subtract again.
func (s *Service) outstandingRepairBalance(ctx context.Context, tenantID, repairID uuid.UUID) (float64, error) {
	_, balance, _, _, err := s.RepairPaymentContext(ctx, tenantID, repairID)
	return balance, err
}

// TryMarkCollectedIfSettled notes on the timeline that a finished job is paid up
// and only waiting for the customer to walk in.
//
// It deliberately does not advance the job to collected. Money clearing and a
// device leaving the counter are different events, and treating a payment as a
// collection loses the only record of who actually took the device — which is
// exactly what gets disputed later. RecordHandover owns that transition.
func (s *Service) TryMarkCollectedIfSettled(ctx context.Context, tenantID, repairID, actorID, corrID uuid.UUID) {
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&status)
	if err != nil || status != StatusCompleted {
		return
	}
	due, err := s.outstandingRepairBalance(ctx, tenantID, repairID)
	if err != nil || due > 0.009 {
		return
	}
	// One note per job, however many payments land against it.
	var already bool
	_ = s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM repair.repair_status_events
			WHERE tenant_id = $1 AND repair_job_id = $2 AND note = $3
		)`, tenantID, repairID, paidAwaitingCollectionNote).Scan(&already)
	if already {
		return
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairID, StatusCompleted, paidAwaitingCollectionNote, actorID, corrID)
}

const paidAwaitingCollectionNote = "Paid in full — waiting for the customer to collect"

func (s *Service) DeleteRepair(ctx context.Context, tenantID, repairID, actorID uuid.UUID) error {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, repairID).
		Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("repair not found")
		}
		return err
	}
	if !CanDelete(status) {
		return fmt.Errorf("cannot delete completed repair")
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET deleted_at = now(), deleted_by = $3, updated_at = now(), updated_by = $3
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`,
		tenantID, repairID, actorID)
	return err
}

func (s *Service) ListTrashedRepairs(ctx context.Context, tenantID uuid.UUID) ([]RepairJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.job_number, j.job_code, j.branch_id, j.customer_id, c.full_name,
		       j.device_id, j.technician_id, j.status, j.problem_summary,
		       COALESCE(j.labor_amount, 0)::float8, j.created_at, j.deleted_at, j.deleted_by
		FROM repair.repair_jobs j
		LEFT JOIN repair.customers c ON c.id = j.customer_id
		WHERE j.tenant_id = $1 AND j.deleted_at IS NOT NULL
		ORDER BY j.deleted_at DESC
		LIMIT 250`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RepairJob, 0)
	for rows.Next() {
		var job RepairJob
		if err := rows.Scan(
			&job.ID, &job.JobNumber, &job.JobCode, &job.BranchID, &job.CustomerID, &job.CustomerName,
			&job.DeviceID, &job.TechnicianID, &job.Status, &job.ProblemSummary,
			&job.LaborAmount, &job.CreatedAt, &job.DeletedAt, &job.DeletedBy,
		); err != nil {
			return nil, err
		}
		items = append(items, job)
	}
	return items, rows.Err()
}

func (s *Service) RestoreRepair(ctx context.Context, tenantID, repairID, actorID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET deleted_at = NULL, deleted_by = NULL, updated_at = now(), updated_by = $3
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL`,
		tenantID, repairID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("trashed repair not found")
	}
	return nil
}

func (s *Service) PurgeRepair(ctx context.Context, tenantID, repairID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM repair.repair_jobs
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL`,
		tenantID, repairID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("trashed repair not found")
	}
	return nil
}

func (s *Service) loadTimeline(ctx context.Context, tenantID, repairID uuid.UUID) ([]StatusEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT status, note, created_at, created_by FROM repair.repair_status_events
		WHERE tenant_id = $1 AND repair_job_id = $2 ORDER BY created_at ASC`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var timeline []StatusEvent
	for rows.Next() {
		var e StatusEvent
		if err := rows.Scan(&e.Status, &e.Note, &e.At, &e.By); err != nil {
			return nil, err
		}
		timeline = append(timeline, e)
	}
	return timeline, nil
}

func (s *Service) publish(eventType string, tenantID, branchID, actorID, corrID uuid.UUID, payload map[string]any) {
	if s.bus == nil {
		return
	}
	env := events.New(eventType, tenantID, corrID, payload)
	if branchID != uuid.Nil {
		env.BranchID = &branchID
	}
	if actorID != uuid.Nil {
		env.ActorID = &actorID
	}
	s.bus.Publish(env)
}
