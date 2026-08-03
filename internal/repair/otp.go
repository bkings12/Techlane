package repair

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	otpTTL         = 10 * time.Minute
	otpMaxAttempts = 5
	otpMinInterval = 60 * time.Second
	otpMaxPerHour  = 5
	sessionTTL     = 30 * 24 * time.Hour
)

type OTPChallengeMeta struct {
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	Attempts   int
}

// CanRequestOTP enforces cooldown and hourly rate limits.
func CanRequestOTP(recent []OTPChallengeMeta, now time.Time) error {
	var hourCount int
	for _, ch := range recent {
		if now.Sub(ch.CreatedAt) < otpMinInterval {
			return fmt.Errorf("please wait before requesting another code")
		}
		if now.Sub(ch.CreatedAt) < time.Hour {
			hourCount++
		}
	}
	if hourCount >= otpMaxPerHour {
		return fmt.Errorf("too many OTP requests; try again later")
	}
	return nil
}

// CanVerifyOTP checks expiry, attempts, and consumption before comparing codes.
func CanVerifyOTP(ch OTPChallengeMeta, now time.Time) error {
	if ch.ConsumedAt != nil {
		return fmt.Errorf("code already used")
	}
	if !ch.ExpiresAt.After(now) {
		return fmt.Errorf("code expired")
	}
	if ch.Attempts >= otpMaxAttempts {
		return fmt.Errorf("too many attempts")
	}
	return nil
}

func hashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateOTPCode() (string, error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

func generateSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

type CustomerSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Customer  *Customer `json:"customer"`
}

func (s *Service) RequestOTP(ctx context.Context, tenantID uuid.UUID, phone string) error {
	phoneE164, err := NormalizePhone(phone)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rows, err := s.pool.Query(ctx, `
		SELECT created_at, expires_at, consumed_at, attempts
		FROM repair.customer_otp_challenges
		WHERE tenant_id = $1 AND phone_e164 = $2 AND purpose = 'login' AND created_at > $3
		ORDER BY created_at DESC LIMIT 10`,
		tenantID, phoneE164, now.Add(-time.Hour))
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
	id := uuid.New()
	expiresAt := now.Add(otpTTL)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.customer_otp_challenges (id, tenant_id, phone_e164, code_hash, attempts, expires_at, created_at, purpose)
		VALUES ($1, $2, $3, $4, 0, $5, $6, 'login')`,
		id, tenantID, phoneE164, hashSecret(code), expiresAt, now)
	if err != nil {
		return err
	}
	sender := s.resolveSMSSender(ctx, tenantID)
	return sender.SendOTP(ctx, phoneE164, code)
}

func (s *Service) VerifyOTP(ctx context.Context, tenantID uuid.UUID, phone, code string) (*CustomerSession, error) {
	phoneE164, err := NormalizePhone(phone)
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if len(code) < 4 {
		return nil, fmt.Errorf("code required")
	}
	now := time.Now().UTC()

	var (
		challengeID uuid.UUID
		codeHash    string
		meta        OTPChallengeMeta
	)
	err = s.pool.QueryRow(ctx, `
		SELECT id, code_hash, attempts, expires_at, consumed_at, created_at
		FROM repair.customer_otp_challenges
		WHERE tenant_id = $1 AND phone_e164 = $2 AND purpose = 'login'
		ORDER BY created_at DESC LIMIT 1`, tenantID, phoneE164).
		Scan(&challengeID, &codeHash, &meta.Attempts, &meta.ExpiresAt, &meta.ConsumedAt, &meta.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("no active code; request a new OTP")
	}
	if err != nil {
		return nil, err
	}
	if err := CanVerifyOTP(meta, now); err != nil {
		return nil, err
	}
	if hashSecret(code) != codeHash {
		_, _ = s.pool.Exec(ctx, `
			UPDATE repair.customer_otp_challenges SET attempts = attempts + 1 WHERE id = $1`, challengeID)
		return nil, fmt.Errorf("invalid code")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE repair.customer_otp_challenges SET consumed_at = $1 WHERE id = $2 AND consumed_at IS NULL`,
		now, challengeID)
	if err != nil {
		return nil, err
	}

	customer, err := s.findOrCreateCustomerByPhone(ctx, tx, tenantID, phoneE164)
	if err != nil {
		return nil, err
	}

	token, err := generateSessionToken()
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(sessionTTL)
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.customer_sessions (id, tenant_id, customer_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), tenantID, customer.ID, hashSecret(token), expiresAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &CustomerSession{Token: token, ExpiresAt: expiresAt, Customer: customer}, nil
}

func (s *Service) findOrCreateCustomerByPhone(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, phoneE164 string) (*Customer, error) {
	variants := PhoneMatchVariants(phoneE164)
	var c Customer
	err := tx.QueryRow(ctx, `
		SELECT id, full_name, phone, email FROM repair.customers
		WHERE tenant_id = $1
		  AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
		ORDER BY
		  (SELECT COUNT(*) FROM repair.repair_jobs j WHERE j.customer_id = repair.customers.id) DESC,
		  created_at ASC
		LIMIT 1`, tenantID, variants).
		Scan(&c.ID, &c.FullName, &c.Phone, &c.Email)
	if err == nil {
		// Fold any 07… / 254… duplicates into this keeper so jobs appear in the customer app.
		if mErr := s.mergeCustomerPhoneDuplicates(ctx, tx, tenantID, c.ID, phoneE164, variants); mErr != nil {
			return nil, mErr
		}
		if c.Phone == nil || *c.Phone != phoneE164 {
			_, _ = tx.Exec(ctx, `UPDATE repair.customers SET phone = $1, updated_at = now() WHERE id = $2`, phoneE164, c.ID)
			c.Phone = &phoneE164
		}
		return &c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	id := uuid.New()
	name := "Customer " + phoneE164
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.customers (id, tenant_id, full_name, phone)
		VALUES ($1, $2, $3, $4)`, id, tenantID, name, phoneE164)
	if err != nil {
		// Unique race against an equivalent format — re-resolve.
		if existing, qErr := s.findCustomerByPhoneVariantsTx(ctx, tx, tenantID, variants); qErr == nil {
			if mErr := s.mergeCustomerPhoneDuplicates(ctx, tx, tenantID, existing.ID, phoneE164, variants); mErr != nil {
				return nil, mErr
			}
			return existing, nil
		}
		return nil, err
	}
	return &Customer{ID: id, FullName: name, Phone: &phoneE164}, nil
}

func (s *Service) findCustomerByPhoneVariantsTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, variants []string) (*Customer, error) {
	var c Customer
	err := tx.QueryRow(ctx, `
		SELECT id, full_name, phone, email FROM repair.customers
		WHERE tenant_id = $1
		  AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = ANY($2::text[])
		ORDER BY
		  (SELECT COUNT(*) FROM repair.repair_jobs j WHERE j.customer_id = repair.customers.id) DESC,
		  created_at ASC
		LIMIT 1`, tenantID, variants).
		Scan(&c.ID, &c.FullName, &c.Phone, &c.Email)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// mergeCustomerPhoneDuplicates reassigns jobs/devices/sessions from equivalent phone rows (07 vs 254) onto keeper.
func (s *Service) mergeCustomerPhoneDuplicates(ctx context.Context, tx pgx.Tx, tenantID, keeperID uuid.UUID, phoneE164 string, variants []string) error {
	rows, err := tx.Query(ctx, `
		SELECT id FROM repair.customers
		WHERE tenant_id = $1
		  AND id <> $2
		  AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') = ANY($3::text[])`,
		tenantID, keeperID, variants)
	if err != nil {
		return err
	}
	defer rows.Close()
	var dupes []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		dupes = append(dupes, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, dupe := range dupes {
		if _, err := tx.Exec(ctx, `UPDATE repair.repair_jobs SET customer_id = $1 WHERE customer_id = $2`, keeperID, dupe); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE repair.devices SET customer_id = $1 WHERE customer_id = $2`, keeperID, dupe); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE repair.customer_sessions SET customer_id = $1 WHERE customer_id = $2`, keeperID, dupe); err != nil {
			return err
		}
		// Clear phone first so unique digit index does not block delete when formats collide mid-merge.
		if _, err := tx.Exec(ctx, `UPDATE repair.customers SET phone = NULL WHERE id = $1`, dupe); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM repair.customers WHERE id = $1`, dupe); err != nil {
			return err
		}
	}
	_, _ = tx.Exec(ctx, `UPDATE repair.customers SET phone = $1, updated_at = now() WHERE id = $2`, phoneE164, keeperID)
	return nil
}

func (s *Service) AuthenticateCustomer(ctx context.Context, tenantID uuid.UUID, token string) (*Customer, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("customer authentication required")
	}
	var c Customer
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.full_name, c.phone, c.email
		FROM repair.customer_sessions cs
		JOIN repair.customers c ON c.id = cs.customer_id AND c.tenant_id = cs.tenant_id
		WHERE cs.tenant_id = $1 AND cs.token_hash = $2 AND cs.expires_at > now()`,
		tenantID, hashSecret(token)).
		Scan(&c.ID, &c.FullName, &c.Phone, &c.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid or expired customer session")
	}
	return &c, err
}

func (s *Service) LogoutCustomer(ctx context.Context, tenantID uuid.UUID, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("customer authentication required")
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM repair.customer_sessions
		WHERE tenant_id = $1 AND token_hash = $2`,
		tenantID, hashSecret(token))
	return err
}

func (s *Service) DefaultTenantID(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM identity.tenants ORDER BY created_at LIMIT 1`).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("no tenant configured")
	}
	return id, nil
}

// CustomerOwnsRepair returns nil when the customer owns the repair job.
func CustomerOwnsRepair(jobCustomerID *uuid.UUID, customerID uuid.UUID) bool {
	return jobCustomerID != nil && *jobCustomerID == customerID
}

func (s *Service) AssertCustomerOwnsRepair(ctx context.Context, tenantID, customerID, repairID uuid.UUID) error {
	var jobCustomerID *uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT customer_id FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&jobCustomerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("repair not found")
	}
	if err != nil {
		return err
	}
	if !CustomerOwnsRepair(jobCustomerID, customerID) {
		return fmt.Errorf("repair not found")
	}
	return nil
}

func (s *Service) ListCustomerRepairs(ctx context.Context, tenantID, customerID uuid.UUID) ([]RepairJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.job_number, j.job_code, j.branch_id, j.customer_id, c.full_name, j.device_id, j.technician_id,
		       j.status, j.problem_summary, COALESCE(j.labor_amount, 0)::float8, j.created_at
		FROM repair.repair_jobs j
		LEFT JOIN repair.customers c ON c.id = j.customer_id
		WHERE j.tenant_id = $1 AND j.customer_id = $2
		ORDER BY j.created_at DESC LIMIT 100`, tenantID, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RepairJob, 0)
	for rows.Next() {
		var j RepairJob
		if err := rows.Scan(
			&j.ID, &j.JobNumber, &j.JobCode, &j.BranchID, &j.CustomerID, &j.CustomerName,
			&j.DeviceID, &j.TechnicianID, &j.Status, &j.ProblemSummary, &j.LaborAmount, &j.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, j)
	}
	return items, rows.Err()
}

func (s *Service) GetCustomerRepair(ctx context.Context, tenantID, customerID, repairID uuid.UUID) (*RepairJob, []RepairEstimate, []PublicReceipt, error) {
	if err := s.AssertCustomerOwnsRepair(ctx, tenantID, customerID, repairID); err != nil {
		return nil, nil, nil, err
	}
	job, err := s.GetRepair(ctx, tenantID, repairID)
	if err != nil {
		return nil, nil, nil, err
	}
	if job.Device != nil {
		job.Device.IMEI = nil
		job.Device.SerialNumber = nil
	}
	estimates, err := s.ListEstimates(ctx, tenantID, repairID)
	if err != nil {
		return nil, nil, nil, err
	}
	receipts, err := s.PublicRepairReceipts(ctx, tenantID, repairID)
	if err != nil {
		return nil, nil, nil, err
	}
	return job, estimates, receipts, nil
}

// RepairPaymentContext returns balance due and branch for customer STK payments.
func (s *Service) RepairPaymentContext(ctx context.Context, tenantID, repairID uuid.UUID) (branchID uuid.UUID, balance float64, defaultPhone string, accountRef string, err error) {
	branchID, balance, _, defaultPhone, accountRef, err = s.repairPaymentAmounts(ctx, tenantID, repairID)
	return branchID, balance, defaultPhone, accountRef, err
}

// RepairOutstanding returns remaining balance. enforceable is false when the job
// has no priced total yet (deposits may still be recorded).
func (s *Service) RepairOutstanding(ctx context.Context, tenantID, repairID uuid.UUID) (outstanding float64, enforceable bool, err error) {
	_, balance, total, _, _, err := s.repairPaymentAmounts(ctx, tenantID, repairID)
	if err != nil {
		return 0, false, err
	}
	return balance, total > 0.009, nil
}

func (s *Service) repairPaymentAmounts(ctx context.Context, tenantID, repairID uuid.UUID) (branchID uuid.UUID, balance, total float64, defaultPhone, accountRef string, err error) {
	var labor float64
	var customerID *uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT branch_id, COALESCE(labor_amount, 0)::float8, customer_id, job_code
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, repairID).
		Scan(&branchID, &labor, &customerID, &accountRef)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, 0, "", "", fmt.Errorf("repair not found")
	}
	if err != nil {
		return uuid.Nil, 0, 0, "", "", err
	}

	total = labor
	var estLabor, estParts float64
	estErr := s.pool.QueryRow(ctx, `
		SELECT labor_amount::float8, parts_amount::float8
		FROM repair.repair_estimates
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status = 'approved'
		ORDER BY decided_at DESC NULLS LAST, created_at DESC LIMIT 1`, tenantID, repairID).
		Scan(&estLabor, &estParts)
	if estErr == nil {
		total = estLabor + estParts
	} else if !errors.Is(estErr, pgx.ErrNoRows) {
		return uuid.Nil, 0, 0, "", "", estErr
	}

	saleExtra, err := s.saleLinesTotal(ctx, tenantID, repairID)
	if err != nil {
		return uuid.Nil, 0, 0, "", "", err
	}
	total += saleExtra

	var paid float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(p.amount), 0)::float8
		FROM payments.payments p
		JOIN payments.payment_allocations a ON a.payment_id = p.id
		WHERE p.tenant_id = $1 AND a.payable_type = 'repair' AND a.payable_id = $2
		  AND p.status IN ('allocated', 'confirmed', 'provisional')`, tenantID, repairID).Scan(&paid)
	if err != nil {
		return uuid.Nil, 0, 0, "", "", err
	}
	balance = total - paid
	if balance < 0 {
		balance = 0
	}
	if customerID != nil {
		var phone *string
		_ = s.pool.QueryRow(ctx, `SELECT phone FROM repair.customers WHERE id = $1`, *customerID).Scan(&phone)
		if phone != nil {
			if n, nerr := NormalizePhone(*phone); nerr == nil {
				defaultPhone = n
			} else {
				defaultPhone = *phone
			}
		}
	}
	return branchID, balance, total, defaultPhone, accountRef, nil
}
