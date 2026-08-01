package notify

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
	"github.com/techlane/techlane/packages/pkg/events"
)

const (
	ChannelSMS      = "sms"
	ChannelWhatsApp = "whatsapp"
	StatusPending   = "pending"
	StatusSent      = "sent"
	StatusFailed    = "failed"

	maxAttempts = 5
	batchSize   = 20
)

type SMSSender interface {
	SendMessage(ctx context.Context, phoneE164, message string) error
}

type TenantSMSResolver func(ctx context.Context, tenantID uuid.UUID) SMSSender

// WhatsAppRouter decides channel preference and delivers WhatsApp messages.
type WhatsAppRouter interface {
	ShouldNotify(ctx context.Context, tenantID uuid.UUID, audience string) (useWA, alsoSMS bool)
	SendMessage(ctx context.Context, tenantID uuid.UUID, phone, message string) error
	RememberPending(ctx context.Context, tenantID uuid.UUID, phone, actionType string, refID uuid.UUID, repairJobID *uuid.UUID, jobCode string, ttl time.Duration) error
}

type Service struct {
	pool       *pgxpool.Pool
	resolveSMS TenantSMSResolver
	whatsapp   WhatsAppRouter
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) SetSMSResolver(resolver TenantSMSResolver) {
	s.resolveSMS = resolver
}

func (s *Service) SetWhatsApp(router WhatsAppRouter) {
	s.whatsapp = router
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

// SendCustomSMS queues a one-off SMS whose body is taken as-is (no template rendering).
func (s *Service) SendCustomSMS(ctx context.Context, tenantID uuid.UUID, phone, body string, repairID, customerID *uuid.UUID) (uuid.UUID, error) {
	phone = strings.TrimSpace(phone)
	body = strings.TrimSpace(body)
	if phone == "" {
		return uuid.Nil, fmt.Errorf("phone number required")
	}
	if body == "" {
		return uuid.Nil, fmt.Errorf("message cannot be empty")
	}
	payload := map[string]any{"body": body}
	if repairID != nil {
		payload["repair_job_id"] = repairID.String()
	}
	if customerID != nil {
		payload["customer_id"] = customerID.String()
	}
	return s.Enqueue(ctx, EnqueueInput{
		TenantID:    tenantID,
		Channel:     ChannelSMS,
		Recipient:   phone,
		TemplateKey: "custom",
		Payload:     payload,
	})
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
		if templateKey == "custom" {
			body, _ := payload["body"].(string)
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("empty message body")
			}
			if s.resolveSMS == nil {
				return fmt.Errorf("SMS resolver not configured")
			}
			sender := s.resolveSMS(ctx, tenantID)
			if sender == nil {
				return fmt.Errorf("SMS provider not configured")
			}
			return sender.SendMessage(ctx, recipient, body)
		}
		custom, _ := s.loadTemplateBody(ctx, tenantID, templateKey)
		message, err := RenderTemplateWithOverride(templateKey, custom, payload)
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
	case ChannelWhatsApp:
		custom, _ := s.loadTemplateBody(ctx, tenantID, templateKey)
		// Interactive WhatsApp copy (YES/NO, QUOTE) — do not reuse SMS portal CTAs.
		if waBody, ok := DefaultWhatsAppBody(templateKey); ok {
			custom = waBody
		}
		message, err := RenderTemplateWithOverride(templateKey, custom, payload)
		if err != nil {
			return err
		}
		if s.whatsapp == nil {
			return fmt.Errorf("WhatsApp not configured")
		}
		return s.whatsapp.SendMessage(ctx, tenantID, recipient, message)
	default:
		return fmt.Errorf("unsupported channel %q", channel)
	}
}

// enqueueCustomerOrSupplier picks WhatsApp and/or SMS based on owner settings.
func (s *Service) enqueueReachable(
	ctx context.Context,
	tenantID uuid.UUID,
	audience, phone, templateKey string,
	payload map[string]any,
) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return
	}
	useWA, alsoSMS := false, true
	if s.whatsapp != nil {
		useWA, alsoSMS = s.whatsapp.ShouldNotify(ctx, tenantID, audience)
	}
	if useWA {
		_, _ = s.Enqueue(ctx, EnqueueInput{
			TenantID: tenantID, Channel: ChannelWhatsApp, Recipient: phone,
			TemplateKey: templateKey, Payload: payload,
		})
		s.maybeRememberWhatsAppPending(ctx, tenantID, phone, templateKey, payload)
	}
	if alsoSMS || !useWA {
		_, _ = s.Enqueue(ctx, EnqueueInput{
			TenantID: tenantID, Channel: ChannelSMS, Recipient: phone,
			TemplateKey: templateKey, Payload: payload,
		})
	}
}

func (s *Service) maybeRememberWhatsAppPending(ctx context.Context, tenantID uuid.UUID, phone, templateKey string, payload map[string]any) {
	if s.whatsapp == nil {
		return
	}
	jobCode, _ := payload["job_code"].(string)
	var repairID *uuid.UUID
	if rid := parseUUIDPayload(payload, "repair_job_id"); rid != uuid.Nil {
		repairID = &rid
	}
	switch templateKey {
	case "estimate.pending":
		if eid := parseUUIDPayload(payload, "estimate_id"); eid != uuid.Nil {
			_ = s.whatsapp.RememberPending(ctx, tenantID, phone, "estimate_decide", eid, repairID, jobCode, 72*time.Hour)
		}
	case "part_request.created":
		if pid := parseUUIDPayload(payload, "part_request_id"); pid != uuid.Nil {
			_ = s.whatsapp.RememberPending(ctx, tenantID, phone, "part_quote", pid, repairID, jobCode, 72*time.Hour)
		}
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
	bus.Subscribe("repair.created", func(e events.Envelope) {
		s.onRepairCreated(context.Background(), e, lookup)
	})
	bus.Subscribe("part_request.created", func(e events.Envelope) {
		s.onPartRequestCreated(context.Background(), e, lookup)
	})
	bus.Subscribe("repair.status_changed", func(e events.Envelope) {
		s.onRepairStatusChanged(context.Background(), e, lookup)
	})
	bus.Subscribe("repair.completed", func(e events.Envelope) {
		s.onRepairCompleted(context.Background(), e, lookup)
	})
	bus.Subscribe("repair.collected", func(e events.Envelope) {
		s.onRepairCollected(context.Background(), e, lookup)
	})
	bus.Subscribe("estimate.pending", func(e events.Envelope) {
		s.onEstimatePending(context.Background(), e, lookup)
	})
	bus.Subscribe("payment.confirmed", func(e events.Envelope) {
		s.onPaymentConfirmed(context.Background(), e, lookup)
	})
}

type RepairLookup interface {
	RepairNotifyInfo(ctx context.Context, tenantID, repairID uuid.UUID) (RepairNotifyInfo, error)
	RepairNotifyContext(ctx context.Context, tenantID, repairID uuid.UUID) (jobCode, phone, shopName string, err error)
	PaymentNotifyContext(ctx context.Context, tenantID, paymentID uuid.UUID) (repairID *uuid.UUID, jobCode, phone, amount, currency string, err error)
}

func payloadFromRepair(info RepairNotifyInfo) map[string]any {
	return map[string]any{
		"shop_name":        info.ShopName,
		"customer_name":    info.CustomerName,
		"job_code":         info.JobCode,
		"pickup_code":      info.PickupCode,
		"device_label":     info.DeviceLabel,
		"problem_summary":  info.ProblemSummary,
		"status":           info.Status,
		"status_label":     PrettyStatus(info.Status),
		"branch_name":      info.BranchName,
		"branch_location":  info.BranchLocation,
		"pickup_place":     info.PickupPlace,
		"currency":         info.Currency,
		"labor_amount":     info.LaborAmount,
		"balance":          info.Balance,
		"pricing_line":     info.PricingLine,
		"promised_by":      info.PromisedBy,
		"wait_minutes":     info.WaitMinutes,
		"wait_line":        info.WaitLine,
		"customer_waiting": info.CustomerWaiting,
	}
}

func (s *Service) onRepairCreated(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID := parseUUIDPayload(e.Payload, "repair_job_id")
	if repairID == uuid.Nil {
		return
	}
	info, err := lookup.RepairNotifyInfo(ctx, e.TenantID, repairID)
	if err != nil || info.Phone == "" {
		return
	}
	if jc, ok := e.Payload["job_code"].(string); ok && strings.TrimSpace(jc) != "" {
		info.JobCode = jc
	}
	if v, ok := e.Payload["pickup_code"].(string); ok && strings.TrimSpace(v) != "" {
		info.PickupCode = v
	}
	if v, ok := e.Payload["problem_summary"].(string); ok && strings.TrimSpace(v) != "" {
		info.ProblemSummary = v
	}
	if v, ok := e.Payload["labor_amount"]; ok {
		switch t := v.(type) {
		case float64:
			info.LaborAmount = t
		case int:
			info.LaborAmount = float64(t)
		}
	}
	if v, ok := e.Payload["customer_waiting"].(bool); ok {
		info.CustomerWaiting = v
	}
	if v, ok := e.Payload["estimated_wait_minutes"]; ok {
		switch t := v.(type) {
		case float64:
			info.WaitMinutes = int(t)
		case int:
			info.WaitMinutes = t
		}
	}
	_, _, _ = s.enqueueRepairIntakeSMS(ctx, e.TenantID, e.BranchID, repairID, info)
}

// ResendRepairIntakeSMS re-queues the intake / wait-bench SMS using the customer's
// current phone on file. Use after correcting a mistyped number.
func (s *Service) ResendRepairIntakeSMS(ctx context.Context, tenantID, repairID uuid.UUID, lookup RepairLookup) (templateKey, phone string, err error) {
	info, err := lookup.RepairNotifyInfo(ctx, tenantID, repairID)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(info.Phone) == "" {
		return "", "", fmt.Errorf("customer has no phone number on file")
	}
	var branchID *uuid.UUID
	templateKey, phone, err = s.enqueueRepairIntakeSMS(ctx, tenantID, branchID, repairID, info)
	return templateKey, phone, err
}

func (s *Service) enqueueRepairIntakeSMS(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, repairID uuid.UUID, info RepairNotifyInfo) (templateKey, phone string, err error) {
	if info.LaborAmount > 0 {
		info.PricingLine = fmt.Sprintf("Quoted %s %.0f.", info.Currency, info.LaborAmount)
	} else if info.PricingLine == "" {
		info.PricingLine = "We'll send an estimate after diagnosis."
	}
	if info.PromisedBy != "" && !info.CustomerWaiting && !strings.Contains(info.PricingLine, "Promised by") {
		info.PricingLine += " Promised by " + info.PromisedBy + "."
	}
	if info.CustomerWaiting && info.WaitMinutes > 0 {
		info.WaitLine = fmt.Sprintf("Please wait at the wait bench — about %d minutes.", info.WaitMinutes)
	} else if info.CustomerWaiting && info.WaitLine == "" {
		info.WaitLine = "Please wait at the wait bench."
	}
	payload := payloadFromRepair(info)
	payload["repair_job_id"] = repairID.String()
	templateKey = "repair.created"
	inboxTitle := fmt.Sprintf("Job created · %s", info.JobCode)
	inboxBody := fmt.Sprintf("Customer notified for new job %s (pickup %s).", info.JobCode, info.PickupCode)
	if info.CustomerWaiting {
		templateKey = "repair.wait_bench"
		inboxTitle = fmt.Sprintf("Wait bench · %s", info.JobCode)
		if info.WaitMinutes > 0 {
			inboxBody = fmt.Sprintf("Customer waiting at the bench for %s (~%d min).", info.JobCode, info.WaitMinutes)
		} else {
			inboxBody = fmt.Sprintf("Customer waiting at the bench for %s.", info.JobCode)
		}
	}
	phone = strings.TrimSpace(info.Phone)
	s.enqueueReachable(ctx, tenantID, "customer", phone, templateKey, payload)
	_ = s.PostStaffInbox(ctx, tenantID, branchID, inboxTitle, inboxBody, templateKey, payload)
	return templateKey, phone, nil
}

func (s *Service) onPartRequestCreated(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	phone, _ := e.Payload["supplier_phone"].(string)
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return
	}
	shopName := "TechLane"
	deviceLabel := "device"
	branchName := "the shop"
	if repairID := parseUUIDPayload(e.Payload, "repair_job_id"); repairID != uuid.Nil {
		if info, err := lookup.RepairNotifyInfo(ctx, e.TenantID, repairID); err == nil {
			if info.ShopName != "" {
				shopName = info.ShopName
			}
			deviceLabel = info.DeviceLabel
			branchName = info.BranchName
		}
	}
	jobCode, _ := e.Payload["job_code"].(string)
	if jobCode == "" {
		jobCode = "job"
	}
	desc, _ := e.Payload["description"].(string)
	if desc == "" {
		desc = "part"
	}
	payload := map[string]any{
		"shop_name": shopName, "job_code": jobCode,
		"description": desc, "quantity": e.Payload["quantity"],
		"supplier_name":   e.Payload["supplier_name"],
		"part_request_id": e.Payload["part_request_id"],
		"repair_job_id":   e.Payload["repair_job_id"],
		"device_label":    deviceLabel,
		"branch_name":     branchName,
	}
	s.enqueueReachable(ctx, e.TenantID, "supplier", phone, "part_request.created", payload)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Part request · %s", jobCode),
		fmt.Sprintf("Supplier notified for %s on job %s.", desc, jobCode),
		"part_request.created", payload)
}

func (s *Service) onRepairStatusChanged(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID, to := parseRepairEvent(e)
	if repairID == uuid.Nil || to == "" {
		return
	}
	if to == "completed" || to == "collected" {
		return // dedicated templates handle these
	}
	info, err := lookup.RepairNotifyInfo(ctx, e.TenantID, repairID)
	if err != nil {
		return
	}
	info.Status = to
	payload := payloadFromRepair(info)
	payload["repair_job_id"] = repairID.String()
	payload["status"] = to
	payload["status_label"] = PrettyStatus(to)

	// Staff always see the move on the board.
	title := fmt.Sprintf("Repair %s → %s", info.JobCode, to)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID, title,
		fmt.Sprintf("Job %s status changed to %s.", info.JobCode, PrettyStatus(to)),
		"repair.status_changed", payload)

	// Customers only get SMS for statuses that matter to them — not every bench hop
	// (diagnosed / in progress / QC), which was flooding and arriving out of order.
	if !CustomerSMSOnStatus(to) || info.Phone == "" {
		return
	}
	s.enqueueReachable(ctx, e.TenantID, "customer", info.Phone, "repair.status_changed", payload)
}

func (s *Service) onRepairCompleted(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID, _ := parseRepairEvent(e)
	if repairID == uuid.Nil {
		return
	}
	info, err := lookup.RepairNotifyInfo(ctx, e.TenantID, repairID)
	if err != nil || info.Phone == "" {
		return
	}
	payload := payloadFromRepair(info)
	payload["repair_job_id"] = repairID.String()
	s.enqueueReachable(ctx, e.TenantID, "customer", info.Phone, "repair.ready", payload)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Repair %s ready", info.JobCode),
		fmt.Sprintf("Job %s is ready for collection (balance %s %.0f).", info.JobCode, info.Currency, info.Balance),
		"repair.ready", payload)
}

func (s *Service) onEstimatePending(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID := parseUUIDPayload(e.Payload, "repair_job_id")
	if repairID == uuid.Nil {
		return
	}
	info, err := lookup.RepairNotifyInfo(ctx, e.TenantID, repairID)
	if err != nil || info.Phone == "" {
		return
	}
	payload := payloadFromRepair(info)
	payload["repair_job_id"] = repairID.String()
	total := 0.0
	if v, ok := e.Payload["total_amount"].(float64); ok {
		total = v
	} else {
		labor, parts := 0.0, 0.0
		if v, ok := e.Payload["labor_amount"].(float64); ok {
			labor = v
		}
		if v, ok := e.Payload["parts_amount"].(float64); ok {
			parts = v
		}
		total = labor + parts
	}
	payload["total_amount"] = total
	if eid := parseUUIDPayload(e.Payload, "estimate_id"); eid != uuid.Nil {
		payload["estimate_id"] = eid.String()
	}
	recommendationLine := ""
	if notes, ok := e.Payload["notes"].(string); ok {
		notes = strings.TrimSpace(notes)
		if notes != "" {
			recommendationLine = " Note: " + notes + "."
		}
	}
	payload["recommendation_line"] = recommendationLine
	s.enqueueReachable(ctx, e.TenantID, "customer", info.Phone, "estimate.pending", payload)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Estimate pending · %s", info.JobCode),
		fmt.Sprintf("Customer estimate awaiting approval for job %s (total %s %.0f).", info.JobCode, info.Currency, total),
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
	payload := map[string]any{
		"shop_name": "TechLane", "job_code": jobCode, "amount": amount, "currency": currency,
		"payment_id": paymentID.String(), "balance": "0", "customer_name": "there",
		"device_label": "device", "method": "payment",
	}
	if method, ok := e.Payload["method"].(string); ok && strings.TrimSpace(method) != "" {
		payload["method"] = method
	}
	if repairID != nil {
		if info, iErr := lookup.RepairNotifyInfo(ctx, e.TenantID, *repairID); iErr == nil {
			for k, v := range payloadFromRepair(info) {
				payload[k] = v
			}
			payload["amount"] = amount
			if currency != "" {
				payload["currency"] = currency
			}
		}
		payload["repair_job_id"] = repairID.String()
	}
	s.enqueueReachable(ctx, e.TenantID, "customer", phone, "payment.confirmed", payload)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Payment confirmed · %s", jobCode),
		fmt.Sprintf("Payment of %s %s received for job %s.", currency, amount, jobCode),
		"payment.confirmed", payload)
}

func (s *Service) onRepairCollected(ctx context.Context, e events.Envelope, lookup RepairLookup) {
	repairID := parseUUIDPayload(e.Payload, "repair_job_id")
	if repairID == uuid.Nil {
		return
	}
	info, err := lookup.RepairNotifyInfo(ctx, e.TenantID, repairID)
	if err != nil || info.Phone == "" {
		return
	}
	payload := payloadFromRepair(info)
	payload["repair_job_id"] = repairID.String()
	if who, ok := e.Payload["collected_by"].(string); ok && strings.TrimSpace(who) != "" {
		payload["collected_by"] = who
	} else {
		payload["collected_by"] = "the customer"
	}
	s.enqueueReachable(ctx, e.TenantID, "customer", info.Phone, "repair.collected", payload)
	_ = s.PostStaffInbox(ctx, e.TenantID, e.BranchID,
		fmt.Sprintf("Collected · %s", info.JobCode),
		fmt.Sprintf("Device for %s handed over.", info.JobCode),
		"repair.collected", payload)
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
