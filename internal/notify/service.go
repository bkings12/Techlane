package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/packages/pkg/events"
)

const (
	ChannelSMS   = "sms"
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"

	maxAttempts = 5
	batchSize   = 20
)

type SMSSender interface {
	SendMessage(ctx context.Context, phoneE164, message string) error
}

type TenantSMSResolver func(ctx context.Context, tenantID uuid.UUID) SMSSender

type Service struct {
	pool       *pgxpool.Pool
	resolveSMS TenantSMSResolver
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) SetSMSResolver(resolver TenantSMSResolver) {
	s.resolveSMS = resolver
}

type EnqueueInput struct {
	TenantID    uuid.UUID
	Channel     string
	Recipient   string
	TemplateKey string
	Payload     map[string]any
}

type OutboxRow struct {
	ID          uuid.UUID      `json:"id"`
	TenantID    uuid.UUID      `json:"tenant_id"`
	Channel     string         `json:"channel"`
	Recipient   string         `json:"recipient"`
	TemplateKey string         `json:"template_key,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Status      string         `json:"status"`
	Attempts    int            `json:"attempts"`
	LastError   *string        `json:"last_error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	SentAt      *time.Time     `json:"sent_at,omitempty"`
}

type StaffNotification struct {
	ID          uuid.UUID      `json:"id"`
	TenantID    uuid.UUID      `json:"tenant_id"`
	BranchID    *uuid.UUID     `json:"branch_id,omitempty"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	TemplateKey string         `json:"template_key,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	ReadAt      *time.Time     `json:"read_at,omitempty"`
	AckedAt     *time.Time     `json:"acked_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

func (s *Service) Enqueue(ctx context.Context, in EnqueueInput) (uuid.UUID, error) {
	if in.TenantID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("tenant_id required")
	}
	if in.Channel == "" {
		in.Channel = ChannelSMS
	}
	if in.Recipient == "" {
		return uuid.Nil, fmt.Errorf("recipient required")
	}
	id := uuid.New()
	payloadJSON, err := json.Marshal(in.Payload)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO notify.notification_outbox
			(id, tenant_id, channel, recipient, template_key, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())`,
		id, in.TenantID, in.Channel, in.Recipient, nullIfEmpty(in.TemplateKey), payloadJSON, StatusPending)
	return id, err
}

func (s *Service) PostStaffInbox(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, title, body, templateKey string, payload map[string]any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO notify.staff_inbox (id, tenant_id, branch_id, title, body, template_key, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())`,
		uuid.New(), tenantID, branchID, title, body, nullIfEmpty(templateKey), payloadJSON)
	return err
}

func (s *Service) ListStaffInbox(ctx context.Context, tenantID uuid.UUID, unackedOnly bool, limit int) ([]StaffNotification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, tenant_id, branch_id, title, body, template_key, payload, read_at, acked_at, created_at
		FROM notify.staff_inbox
		WHERE tenant_id = $1`
	args := []any{tenantID}
	if unackedOnly {
		q += ` AND acked_at IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT $2`
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]StaffNotification, 0)
	for rows.Next() {
		var n StaffNotification
		var payloadRaw []byte
		if err := rows.Scan(
			&n.ID, &n.TenantID, &n.BranchID, &n.Title, &n.Body, &n.TemplateKey,
			&payloadRaw, &n.ReadAt, &n.AckedAt, &n.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &n.Payload)
		}
		items = append(items, n)
	}
	return items, rows.Err()
}

func (s *Service) AckStaffInbox(ctx context.Context, tenantID, notificationID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE notify.staff_inbox
		SET acked_at = COALESCE(acked_at, now()), read_at = COALESCE(read_at, now())
		WHERE tenant_id = $1 AND id = $2`, tenantID, notificationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (s *Service) DrainPending(ctx context.Context) (processed, sent, failed int, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, channel, recipient, template_key, payload, attempts
		FROM notify.notification_outbox
		WHERE status = $1
		  AND (next_attempt_at IS NULL OR next_attempt_at <= now())
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`, StatusPending, batchSize)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()

	type job struct {
		id          uuid.UUID
		tenantID    uuid.UUID
		channel     string
		recipient   string
		templateKey string
		payload     map[string]any
		attempts    int
	}
	var jobs []job
	for rows.Next() {
		var j job
		var templateKey *string
		var payloadRaw []byte
		if err := rows.Scan(&j.id, &j.tenantID, &j.channel, &j.recipient, &templateKey, &payloadRaw, &j.attempts); err != nil {
			return 0, 0, 0, err
		}
		if templateKey != nil {
			j.templateKey = *templateKey
		}
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &j.payload)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, 0, err
	}

	for _, j := range jobs {
		processed++
		sendErr := s.deliver(ctx, j.tenantID, j.channel, j.recipient, j.templateKey, j.payload)
		if sendErr == nil {
			sent++
			_, _ = s.pool.Exec(ctx, `
				UPDATE notify.notification_outbox
				SET status = $1, sent_at = now(), last_error = NULL
				WHERE id = $2`, StatusSent, j.id)
			continue
		}
		attempts := j.attempts + 1
		errMsg := sendErr.Error()
		if attempts >= maxAttempts {
			failed++
			_, _ = s.pool.Exec(ctx, `
				UPDATE notify.notification_outbox
				SET status = $1, attempts = $2, last_error = $3
				WHERE id = $4`, StatusFailed, attempts, errMsg, j.id)
		} else {
			backoff := retryDelay(attempts)
			next := time.Now().UTC().Add(backoff)
			_, _ = s.pool.Exec(ctx, `
				UPDATE notify.notification_outbox
				SET attempts = $1, last_error = $2, next_attempt_at = $3
				WHERE id = $4`, attempts, errMsg, next, j.id)
		}
	}
	return processed, sent, failed, nil
}

func (s *Service) deliver(ctx context.Context, tenantID uuid.UUID, channel, recipient, templateKey string, payload map[string]any) error {
	switch channel {
	case ChannelSMS:
		message, err := RenderTemplate(templateKey, payload)
		if err != nil {
			return err
		}
		if s.resolveSMS == nil {
			return fmt.Errorf("SMS resolver not configured")
		}
		sender := s.resolveSMS(ctx, tenantID)
		if sender == nil {
			return fmt.Errorf("SMS provider not configured")
		}
		return sender.SendMessage(ctx, recipient, message)
	default:
		return fmt.Errorf("unsupported channel %q", channel)
	}
}

func retryDelay(attempts int) time.Duration {
	switch attempts {
	case 1:
		return 30 * time.Second
	case 2:
		return 2 * time.Minute
	case 3:
		return 10 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Wire subscribes to domain events and enqueues notifications.
func (s *Service) Wire(bus *events.Bus, lookup RepairLookup) {
	if bus == nil || lookup == nil {
		return
	}
	bus.Subscribe("repair.status_changed", func(e events.Envelope) {
		s.onRepairStatusChanged(context.Background(), e, lookup)
	})
	bus.Subscribe("repair.completed", func(e events.Envelope) {
		s.onRepairCompleted(context.Background(), e, lookup)
	})
	bus.Subscribe("estimate.pending", func(e events.Envelope) {
		s.onEstimatePending(context.Background(), e, lookup)
	})
	bus.Subscribe("payment.confirmed", func(e events.Envelope) {
		s.onPaymentConfirmed(context.Background(), e, lookup)
	})
}

type RepairLookup interface {
	RepairNotifyContext(ctx context.Context, tenantID, repairID uuid.UUID) (jobCode, phone, shopName string, err error)
	PaymentNotifyContext(ctx context.Context, tenantID, paymentID uuid.UUID) (repairID *uuid.UUID, jobCode, phone, amount, currency string, err error)
}

func (s *Service) onRepairStatusChanged(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID, to := parseRepairEvent(e)
	if repairID == uuid.Nil || to == "" {
		return
	}
	if to == "completed" {
		return // repair.completed handles ready SMS
	}
	jobCode, phone, shopName, err := lookup.RepairNotifyContext(ctx, e.TenantID, repairID)
	if err != nil || phone == "" {
		return
	}
	payload := map[string]any{
		"shop_name": shopName, "job_code": jobCode, "status": to,
		"repair_job_id": repairID.String(),
	}
	_, _ = s.Enqueue(ctx, EnqueueInput{
		TenantID: e.TenantID, Channel: ChannelSMS, Recipient: phone,
		TemplateKey: "repair.status_changed", Payload: payload,
	})
	title := fmt.Sprintf("Repair %s → %s", jobCode, to)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID, title,
		fmt.Sprintf("Job %s status changed to %s.", jobCode, to),
		"repair.status_changed", payload)
}

func (s *Service) onRepairCompleted(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID, _ := parseRepairEvent(e)
	if repairID == uuid.Nil {
		return
	}
	jobCode, phone, shopName, err := lookup.RepairNotifyContext(ctx, e.TenantID, repairID)
	if err != nil || phone == "" {
		return
	}
	payload := map[string]any{
		"shop_name": shopName, "job_code": jobCode,
		"repair_job_id": repairID.String(),
	}
	_, _ = s.Enqueue(ctx, EnqueueInput{
		TenantID: e.TenantID, Channel: ChannelSMS, Recipient: phone,
		TemplateKey: "repair.ready", Payload: payload,
	})
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Repair %s ready", jobCode),
		fmt.Sprintf("Job %s is ready for collection.", jobCode),
		"repair.ready", payload)
}

func (s *Service) onEstimatePending(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID := parseUUIDPayload(e.Payload, "repair_job_id")
	if repairID == uuid.Nil {
		return
	}
	jobCode, phone, shopName, err := lookup.RepairNotifyContext(ctx, e.TenantID, repairID)
	if err != nil || phone == "" {
		return
	}
	payload := map[string]any{
		"shop_name": shopName, "job_code": jobCode,
		"repair_job_id": repairID.String(),
	}
	if v, ok := e.Payload["labor_amount"]; ok {
		payload["labor_amount"] = v
	}
	if v, ok := e.Payload["parts_amount"]; ok {
		payload["parts_amount"] = v
	}
	_, _ = s.Enqueue(ctx, EnqueueInput{
		TenantID: e.TenantID, Channel: ChannelSMS, Recipient: phone,
		TemplateKey: "estimate.pending", Payload: payload,
	})
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Estimate pending · %s", jobCode),
		fmt.Sprintf("Customer estimate awaiting approval for job %s.", jobCode),
		"estimate.pending", payload)
}

func (s *Service) onPaymentConfirmed(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	paymentID := parseUUIDPayload(e.Payload, "payment_id")
	if paymentID == uuid.Nil {
		return
	}
	repairID, jobCode, phone, amount, currency, err := lookup.PaymentNotifyContext(ctx, e.TenantID, paymentID)
	if err != nil || phone == "" {
		return
	}
	shopName := "TechLane"
	if repairID != nil {
		_, _, shopName, _ = lookup.RepairNotifyContext(ctx, e.TenantID, *repairID)
	}
	payload := map[string]any{
		"shop_name": shopName, "job_code": jobCode, "amount": amount, "currency": currency,
		"payment_id": paymentID.String(),
	}
	if repairID != nil {
		payload["repair_job_id"] = repairID.String()
	}
	_, _ = s.Enqueue(ctx, EnqueueInput{
		TenantID: e.TenantID, Channel: ChannelSMS, Recipient: phone,
		TemplateKey: "payment.confirmed", Payload: payload,
	})
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Payment confirmed · %s", jobCode),
		fmt.Sprintf("Payment of %s %s received for job %s.", currency, amount, jobCode),
		"payment.confirmed", payload)
}

func parseRepairEvent(e events.Envelope) (repairID uuid.UUID, to string) {
	repairID = parseUUIDPayload(e.Payload, "repair_job_id")
	if v, ok := e.Payload["to"].(string); ok {
		to = v
	}
	return repairID, to
}

func parseUUIDPayload(payload map[string]any, key string) uuid.UUID {
	v, ok := payload[key].(string)
	if !ok || v == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil
	}
	return id
}

var ErrNotFound = errors.New("not found")

func (s *Service) GetOutbox(ctx context.Context, tenantID, id uuid.UUID) (*OutboxRow, error) {
	var row OutboxRow
	var templateKey *string
	var payloadRaw []byte
	var lastError *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, channel, recipient, template_key, payload, status, attempts, last_error, created_at, sent_at
		FROM notify.notification_outbox
		WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&row.ID, &row.TenantID, &row.Channel, &row.Recipient, &templateKey, &payloadRaw,
			&row.Status, &row.Attempts, &lastError, &row.CreatedAt, &row.SentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if templateKey != nil {
		row.TemplateKey = *templateKey
	}
	row.LastError = lastError
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &row.Payload)
	}
	return &row, nil
}
