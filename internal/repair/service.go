package repair

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	BranchID       uuid.UUID     `json:"branch_id"`
	CustomerID     *uuid.UUID    `json:"customer_id,omitempty"`
	CustomerName   *string       `json:"customer_name,omitempty"`
	DeviceID       uuid.UUID     `json:"device_id"`
	TechnicianID   *uuid.UUID    `json:"technician_id,omitempty"`
	Status         string        `json:"status"`
	ProblemSummary string        `json:"problem_summary"`
	LaborAmount    float64       `json:"labor_amount"`
	CreatedAt      time.Time     `json:"created_at"`
	Customer       *Customer     `json:"customer,omitempty"`
	Device         *Device       `json:"device,omitempty"`
	Timeline       []StatusEvent `json:"timeline,omitempty"`
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
	TechnicianID   *uuid.UUID
	ActorID        uuid.UUID
	TenantID       uuid.UUID
	ClientID       *uuid.UUID
	CorrID         uuid.UUID
}

func (s *Service) CreateCustomer(ctx context.Context, in CreateCustomerInput) (*Customer, error) {
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.customers (id, tenant_id, full_name, phone, email, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, in.TenantID, in.FullName, in.Phone, in.Email, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	return &Customer{ID: id, FullName: in.FullName, Phone: in.Phone, Email: in.Email}, nil
}

func (s *Service) ListCustomers(ctx context.Context, tenantID uuid.UUID, q string, limit int) ([]Customer, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, full_name, phone, email FROM repair.customers
		WHERE tenant_id = $1 AND ($2 = '' OR full_name ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC LIMIT $3`, tenantID, q, limit)
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
	phoneE164, err := NormalizePhone(phone)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("repair not found")
	}
	var tenantID, id uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT j.tenant_id, j.id
		FROM repair.repair_jobs j
		JOIN repair.customers c ON c.id = j.customer_id
		WHERE UPPER(j.job_code) = UPPER($1)
		  AND regexp_replace(COALESCE(c.phone, ''), '[^0-9]', '', 'g') = $2
		ORDER BY j.created_at DESC LIMIT 1`, strings.TrimSpace(jobCode), phoneE164).
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
		SELECT DISTINCT p.id, p.method, p.amount::float8, p.currency, p.status, p.provider_ref, p.created_at
		FROM payments.payments p
		JOIN payments.payment_allocations a ON a.payment_id = p.id
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

	tag, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_jobs (id, tenant_id, branch_id, customer_id, device_id, technician_id, status, problem_summary, job_number, job_code, created_by, updated_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $12)
		ON CONFLICT (id) DO NOTHING`,
		id, in.TenantID, in.BranchID, in.CustomerID, in.DeviceID, in.TechnicianID, status, in.ProblemSummary, jobNumber, jobCode, in.ActorID, in.CorrID)
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
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6)`, evID, in.TenantID, id, status, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	job := &RepairJob{
		ID: id, JobNumber: jobNumber, JobCode: jobCode,
		BranchID: in.BranchID, CustomerID: in.CustomerID, DeviceID: in.DeviceID,
		TechnicianID: in.TechnicianID, Status: status, ProblemSummary: in.ProblemSummary,
	}
	s.publish("repair.created", in.TenantID, in.BranchID, in.ActorID, in.CorrID, map[string]any{
		"repair_job_id": id.String(), "job_code": jobCode, "status": status,
	})
	return job, nil
}

type ListRepairsFilter struct {
	BranchID     *uuid.UUID
	Status       string
	TechnicianID *uuid.UUID
	Search       string
}

func (s *Service) ListRepairs(ctx context.Context, tenantID uuid.UUID, f ListRepairsFilter) ([]RepairJob, error) {
	q := `SELECT j.id, j.job_number, j.job_code, j.branch_id, j.customer_id, c.full_name, j.device_id, j.technician_id, j.status, j.problem_summary, COALESCE(j.labor_amount, 0)::float8, j.created_at
		FROM repair.repair_jobs j
		LEFT JOIN repair.customers c ON c.id = j.customer_id
		WHERE j.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if f.BranchID != nil {
		q += fmt.Sprintf(" AND j.branch_id = $%d", n)
		args = append(args, *f.BranchID)
		n++
	}
	if f.Status != "" {
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
		q += fmt.Sprintf(" AND (j.job_code ILIKE '%%' || $%d || '%%' OR j.problem_summary ILIKE '%%' || $%d || '%%' OR c.full_name ILIKE '%%' || $%d || '%%')", n, n, n)
		args = append(args, f.Search)
		n++
	}
	q += " ORDER BY j.created_at DESC LIMIT 100"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RepairJob
	for rows.Next() {
		var j RepairJob
		if err := rows.Scan(&j.ID, &j.JobNumber, &j.JobCode, &j.BranchID, &j.CustomerID, &j.CustomerName, &j.DeviceID, &j.TechnicianID, &j.Status, &j.ProblemSummary, &j.LaborAmount, &j.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, j)
	}
	return items, nil
}

func (s *Service) GetRepair(ctx context.Context, tenantID, id uuid.UUID) (*RepairJob, error) {
	var j RepairJob
	err := s.pool.QueryRow(ctx, `
		SELECT id, job_number, job_code, branch_id, customer_id, device_id, technician_id, status, problem_summary, COALESCE(labor_amount, 0)::float8, created_at
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&j.ID, &j.JobNumber, &j.JobCode, &j.BranchID, &j.CustomerID, &j.DeviceID, &j.TechnicianID, &j.Status, &j.ProblemSummary, &j.LaborAmount, &j.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
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
	return &j, nil
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
		ORDER BY n.created_at DESC`, tenantID, repairID)
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
	s.AdvanceStatusIf(ctx, tenantID, repairID,
		[]string{StatusIntake}, StatusInProgress,
		"Claimed by technician", actorID, corrID)
	return s.GetRepair(ctx, tenantID, repairID)
}

func (s *Service) ChangeStatus(ctx context.Context, tenantID, repairID uuid.UUID, newStatus string, note *string, laborAmount *float64, actorID, corrID uuid.UUID) (*RepairJob, error) {
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

	labor := existingLabor
	if laborAmount != nil {
		labor = *laborAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE repair.repair_jobs SET status = $1, labor_amount = $2, updated_by = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $4 AND id = $5`, newStatus, labor, actorID, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	evID := uuid.New()
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
	if s.completionHook != nil {
		_ = s.completionHook.OnRepairStatusChanged(ctx, tenantID, repairID, newStatus, actorID)
	}
	if newStatus == StatusCompleted {
		s.publish("repair.completed", tenantID, branchID, actorID, corrID, map[string]any{
			"repair_job_id": repairID.String(), "labor_amount": labor,
		})
		if s.commissionHook != nil && technicianID != nil {
			if err := s.commissionHook.AccrueOnRepairCompleted(ctx, tenantID, branchID, repairID, *technicianID, labor, actorID, corrID); err != nil {
				// Status already committed; log via event rather than failing the status change.
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
	s.maybeEnsureWarranty(ctx, tenantID, repairID, newStatus)
	return s.GetRepair(ctx, tenantID, repairID)
}

// AdvanceStatusIf moves a repair forward when from matches current status.
// Best-effort: failures are swallowed so callers (notes, parts, payments) never fail.
func (s *Service) AdvanceStatusIf(ctx context.Context, tenantID, repairID uuid.UUID, from []string, to, note string, actorID, corrID uuid.UUID) {
	if repairID == uuid.Nil || to == "" {
		return
	}
	var current string
	var branchID uuid.UUID
	var technicianID *uuid.UUID
	var labor float64
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id, technician_id, COALESCE(labor_amount, 0)::float8
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&current, &branchID, &technicianID, &labor)
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
		s.publish("repair.completed", tenantID, branchID, actorID, corrID, map[string]any{
			"repair_job_id": repairID.String(), "labor_amount": labor,
		})
		if s.commissionHook != nil && technicianID != nil {
			_ = s.commissionHook.AccrueOnRepairCompleted(ctx, tenantID, branchID, repairID, *technicianID, labor, actorID, corrID)
		}
		if s.completionHook != nil {
			_ = s.completionHook.OnRepairCompleted(ctx, tenantID, repairID, actorID)
		}
	}
	s.maybeEnsureWarranty(ctx, tenantID, repairID, to)
}

// TryMarkCollectedIfSettled advances completed → collected when the repair balance is cleared.
// Counts allocated/confirmed and provisional cash (pending_handover) so counter cash can hand off the device.
func (s *Service) TryMarkCollectedIfSettled(ctx context.Context, tenantID, repairID, actorID, corrID uuid.UUID) {
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairID).Scan(&status)
	if err != nil || status != StatusCompleted {
		return
	}
	_, balance, _, _, err := s.RepairPaymentContext(ctx, tenantID, repairID)
	if err != nil {
		return
	}
	// Also credit provisional cash still awaiting handover — device can leave with paid cash.
	var provisional float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(p.amount), 0)::float8
		FROM payments.payments p
		JOIN payments.payment_allocations a ON a.payment_id = p.id
		WHERE p.tenant_id = $1 AND a.payable_type = 'repair' AND a.payable_id = $2
		  AND p.status = 'pending_handover'`, tenantID, repairID).Scan(&provisional)
	if balance-provisional > 0.009 {
		return
	}
	s.AdvanceStatusIf(ctx, tenantID, repairID,
		[]string{StatusCompleted}, StatusCollected,
		"Balance settled — marked collected", actorID, corrID)
}

func (s *Service) DeleteRepair(ctx context.Context, tenantID, repairID uuid.UUID) error {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, repairID).
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
	_, err = s.pool.Exec(ctx, `DELETE FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, repairID)
	return err
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
