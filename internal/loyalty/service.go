// Package loyalty provides lightweight marketing groundwork on top of the
// core platform: a points-per-customer ledger seeded by domain events
// (repair completions, payments), and outbound webhook subscriptions so
// tenants can wire TechLane into external marketing/CRM tools without
// TechLane needing to know anything about them.
package loyalty

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/packages/pkg/events"
)

var ErrInvalidInput = errors.New("invalid input")

type Service struct {
	pool       *pgxpool.Pool
	httpClient *http.Client
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

// Settings is the per-tenant loyalty configuration.
type Settings struct {
	TenantID                 uuid.UUID `json:"tenant_id"`
	Enabled                  bool      `json:"enabled"`
	PointsPerCompletedRepair int       `json:"points_per_completed_repair"`
	PointsPerCurrencyUnit    float64   `json:"points_per_currency_unit"`
}

type Account struct {
	CustomerID    uuid.UUID `json:"customer_id"`
	PointsBalance int       `json:"points_balance"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type LedgerEntry struct {
	ID            uuid.UUID  `json:"id"`
	Delta         int        `json:"delta"`
	Reason        string     `json:"reason"`
	ReferenceType *string    `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// GetSettings returns the tenant's loyalty configuration, defaulting to a
// disabled program with sensible seed values if none has been saved yet.
func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (*Settings, error) {
	var st Settings
	st.TenantID = tenantID
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, points_per_completed_repair, points_per_currency_unit
		FROM loyalty.settings WHERE tenant_id = $1`, tenantID).
		Scan(&st.Enabled, &st.PointsPerCompletedRepair, &st.PointsPerCurrencyUnit)
	if errors.Is(err, pgx.ErrNoRows) {
		st.Enabled = false
		st.PointsPerCompletedRepair = 10
		st.PointsPerCurrencyUnit = 0
		return &st, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Service) UpdateSettings(ctx context.Context, tenantID uuid.UUID, enabled bool, pointsPerRepair int, pointsPerCurrencyUnit float64) (*Settings, error) {
	if pointsPerRepair < 0 || pointsPerCurrencyUnit < 0 {
		return nil, fmt.Errorf("%w: points rates cannot be negative", ErrInvalidInput)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO loyalty.settings (tenant_id, enabled, points_per_completed_repair, points_per_currency_unit, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (tenant_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			points_per_completed_repair = EXCLUDED.points_per_completed_repair,
			points_per_currency_unit = EXCLUDED.points_per_currency_unit,
			updated_at = now()`,
		tenantID, enabled, pointsPerRepair, pointsPerCurrencyUnit)
	if err != nil {
		return nil, err
	}
	return &Settings{TenantID: tenantID, Enabled: enabled, PointsPerCompletedRepair: pointsPerRepair, PointsPerCurrencyUnit: pointsPerCurrencyUnit}, nil
}

// GetAccount returns a customer's balance (zeroed if they've never earned
// points) plus their most recent ledger activity.
func (s *Service) GetAccount(ctx context.Context, tenantID, customerID uuid.UUID) (*Account, []LedgerEntry, error) {
	acct := &Account{CustomerID: customerID}
	err := s.pool.QueryRow(ctx, `
		SELECT points_balance, updated_at FROM loyalty.accounts WHERE tenant_id = $1 AND customer_id = $2`,
		tenantID, customerID).Scan(&acct.PointsBalance, &acct.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		acct.PointsBalance = 0
		acct.UpdatedAt = time.Time{}
	} else if err != nil {
		return nil, nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, delta, reason, reference_type, reference_id, created_at
		FROM loyalty.ledger WHERE tenant_id = $1 AND customer_id = $2
		ORDER BY created_at DESC LIMIT 50`, tenantID, customerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	entries := make([]LedgerEntry, 0, 16)
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.Delta, &e.Reason, &e.ReferenceType, &e.ReferenceID, &e.CreatedAt); err != nil {
			return nil, nil, err
		}
		entries = append(entries, e)
	}
	return acct, entries, rows.Err()
}

// award credits (or debits, if delta is negative) points to a customer,
// appending a ledger row and updating the running balance atomically.
func (s *Service) award(ctx context.Context, tenantID, customerID uuid.UUID, delta int, reason, refType string, refID *uuid.UUID) error {
	if delta == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO loyalty.accounts (tenant_id, customer_id, points_balance, updated_at)
		VALUES ($1, $2, GREATEST($3, 0), now())
		ON CONFLICT (tenant_id, customer_id) DO UPDATE SET
			points_balance = GREATEST(loyalty.accounts.points_balance + $3, 0),
			updated_at = now()`,
		tenantID, customerID, delta); err != nil {
		return err
	}
	var refTypePtr *string
	if refType != "" {
		refTypePtr = &refType
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO loyalty.ledger (id, tenant_id, customer_id, delta, reason, reference_type, reference_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, customerID, delta, reason, refTypePtr, refID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetEventBus wires accrual (repair completions, payments) and outbound
// webhook dispatch onto the shared in-process event bus.
func (s *Service) SetEventBus(bus *events.Bus) {
	if bus == nil {
		return
	}
	bus.Subscribe("repair.completed", func(e events.Envelope) {
		s.onRepairCompleted(context.Background(), e)
	})
	bus.Subscribe("payment.confirmed", func(e events.Envelope) {
		s.onPaymentConfirmed(context.Background(), e)
	})
	bus.Subscribe("*", func(e events.Envelope) {
		s.dispatchWebhooks(context.Background(), e)
	})
}

func (s *Service) onRepairCompleted(ctx context.Context, e events.Envelope) {
	repairIDStr, _ := e.Payload["repair_job_id"].(string)
	repairID, err := uuid.Parse(repairIDStr)
	if err != nil {
		return
	}
	settings, err := s.GetSettings(ctx, e.TenantID)
	if err != nil || !settings.Enabled || settings.PointsPerCompletedRepair <= 0 {
		return
	}
	var customerID *uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT customer_id FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		e.TenantID, repairID).Scan(&customerID); err != nil || customerID == nil {
		return
	}
	if err := s.award(ctx, e.TenantID, *customerID, settings.PointsPerCompletedRepair, "Repair completed", "repair", &repairID); err != nil {
		slog.Warn("loyalty: award on repair.completed failed", "err", err, "repair_id", repairID)
	}
}

func (s *Service) onPaymentConfirmed(ctx context.Context, e events.Envelope) {
	paymentIDStr, _ := e.Payload["payment_id"].(string)
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return
	}
	settings, err := s.GetSettings(ctx, e.TenantID)
	if err != nil || !settings.Enabled || settings.PointsPerCurrencyUnit <= 0 {
		return
	}
	var payableType string
	var payableID uuid.UUID
	var amount float64
	if err := s.pool.QueryRow(ctx, `
		SELECT a.payable_type, a.payable_id, p.amount::float8
		FROM payments.payment_allocations a
		JOIN payments.payments p ON p.id = a.payment_id
		WHERE p.tenant_id = $1 AND p.id = $2 LIMIT 1`, e.TenantID, paymentID).
		Scan(&payableType, &payableID, &amount); err != nil {
		return
	}
	var customerID *uuid.UUID
	switch payableType {
	case "repair":
		_ = s.pool.QueryRow(ctx, `SELECT customer_id FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
			e.TenantID, payableID).Scan(&customerID)
	case "order":
		_ = s.pool.QueryRow(ctx, `SELECT customer_id FROM sales.orders WHERE tenant_id = $1 AND id = $2`,
			e.TenantID, payableID).Scan(&customerID)
	}
	if customerID == nil {
		return
	}
	points := int(amount * settings.PointsPerCurrencyUnit)
	if points <= 0 {
		return
	}
	if err := s.award(ctx, e.TenantID, *customerID, points, "Payment received", payableType, &payableID); err != nil {
		slog.Warn("loyalty: award on payment.confirmed failed", "err", err, "payment_id", paymentID)
	}
}

// ---- Outbound marketing webhooks ----

type WebhookSubscription struct {
	ID              uuid.UUID  `json:"id"`
	URL             string     `json:"url"`
	EventTypes      []string   `json:"event_types"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
	LastStatus      *string    `json:"last_status,omitempty"`
	// Secret is only ever returned once, at creation time.
	Secret string `json:"secret,omitempty"`
}

func generateSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Service) RegisterWebhook(ctx context.Context, tenantID uuid.UUID, url string, eventTypes []string, actorID *uuid.UUID) (*WebhookSubscription, error) {
	url = strings.TrimSpace(url)
	if url == "" || (!strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://")) {
		return nil, fmt.Errorf("%w: a valid http(s) URL is required", ErrInvalidInput)
	}
	clean := make([]string, 0, len(eventTypes))
	for _, t := range eventTypes {
		t = strings.TrimSpace(t)
		if t != "" {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("%w: at least one event type is required", ErrInvalidInput)
	}
	secret, err := generateSecret()
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO loyalty.webhook_subscriptions (id, tenant_id, url, secret, event_types, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, true, $6)`,
		id, tenantID, url, secret, clean, actorID); err != nil {
		return nil, err
	}
	return &WebhookSubscription{ID: id, URL: url, EventTypes: clean, IsActive: true, CreatedAt: time.Now().UTC(), Secret: secret}, nil
}

func (s *Service) ListWebhooks(ctx context.Context, tenantID uuid.UUID) ([]WebhookSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, event_types, is_active, created_at, last_triggered_at, last_status
		FROM loyalty.webhook_subscriptions WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WebhookSubscription, 0, 8)
	for rows.Next() {
		var w WebhookSubscription
		if err := rows.Scan(&w.ID, &w.URL, &w.EventTypes, &w.IsActive, &w.CreatedAt, &w.LastTriggeredAt, &w.LastStatus); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Service) DeleteWebhook(ctx context.Context, tenantID, id uuid.UUID) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM loyalty.webhook_subscriptions WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// dispatchWebhooks fires an HMAC-signed POST to every active subscription
// whose event_types include e.EventType (or "*"). Best-effort, fire-and-forget:
// a slow or dead endpoint must never block the request that raised the event.
func (s *Service) dispatchWebhooks(ctx context.Context, e events.Envelope) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, secret FROM loyalty.webhook_subscriptions
		WHERE tenant_id = $1 AND is_active AND (event_types @> ARRAY[$2]::text[] OR event_types @> ARRAY['*']::text[])`,
		e.TenantID, e.EventType)
	if err != nil {
		return
	}
	type target struct {
		id     uuid.UUID
		url    string
		secret string
	}
	targets := make([]target, 0, 4)
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.url, &t.secret); err == nil {
			targets = append(targets, t)
		}
	}
	rows.Close()
	if len(targets) == 0 {
		return
	}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	for _, t := range targets {
		go s.deliverWebhook(t.id, t.url, t.secret, body)
	}
}

func (s *Service) deliverWebhook(id uuid.UUID, url, secret string, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TechLane-Signature", "sha256="+sig)

	status := "delivered"
	resp, err := s.httpClient.Do(req)
	if err != nil {
		status = "error: " + err.Error()
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			status = fmt.Sprintf("http_%d", resp.StatusCode)
		}
	}
	if len(status) > 200 {
		status = status[:200]
	}
	_, _ = s.pool.Exec(context.Background(), `
		UPDATE loyalty.webhook_subscriptions SET last_triggered_at = now(), last_status = $1 WHERE id = $2`,
		status, id)
}
