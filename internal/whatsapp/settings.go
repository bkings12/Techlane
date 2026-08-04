package whatsapp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Settings struct {
	TenantID         uuid.UUID `json:"tenant_id"`
	Enabled          bool      `json:"enabled"`
	NotifyCustomers  bool      `json:"notify_customers"`
	NotifySuppliers  bool      `json:"notify_suppliers"`
	AlsoSendSMS      bool      `json:"also_send_sms"`
	ServiceConfigured bool     `json:"service_configured"`
	Connected        bool      `json:"connected"`
	ConnectionStatus string    `json:"connection_status"`
	// Populated only when ConnectionStatus is "reconnect_failed" — the sidecar's
	// circuit breaker tripped after repeated pairing failures, and this is why.
	LastError string    `json:"last_error,omitempty"`
	SessionID string    `json:"session_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpsertSettingsInput struct {
	Enabled         *bool
	NotifyCustomers *bool
	NotifySuppliers *bool
	AlsoSendSMS     *bool
	ActorID         uuid.UUID
}

type Service struct {
	pool   *pgxpool.Pool
	client *Client
	repair RepairActions
	inv    SupplierActions
}

type RepairActions interface {
	DecideEstimateByPhone(ctx context.Context, tenantID uuid.UUID, phone string, approve bool, estimateID *uuid.UUID) (jobCode string, err error)
}

type SupplierActions interface {
	SubmitSupplierQuoteByPhone(ctx context.Context, tenantID uuid.UUID, phone string, unitCost float64, requestID *uuid.UUID) (jobCode string, err error)
	DeclineSupplierRequestByPhone(ctx context.Context, tenantID uuid.UUID, phone string, requestID *uuid.UUID) (jobCode string, err error)
}

func NewService(pool *pgxpool.Pool, client *Client) *Service {
	return &Service{pool: pool, client: client}
}

func (s *Service) SetRepairActions(a RepairActions) { s.repair = a }
func (s *Service) SetSupplierActions(a SupplierActions) { s.inv = a }

func (s *Service) Client() *Client { return s.client }

func SessionID(tenantID uuid.UUID) string { return tenantID.String() }

func (s *Service) GetSettings(ctx context.Context, tenantID uuid.UUID) (*Settings, error) {
	out := &Settings{
		TenantID:          tenantID,
		NotifyCustomers:   true,
		NotifySuppliers:   true,
		SessionID:         SessionID(tenantID),
		ServiceConfigured: s.client != nil && s.client.Configured(),
		ConnectionStatus:  "not_configured",
		UpdatedAt:         time.Now().UTC(),
	}
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, notify_customers, notify_suppliers, also_send_sms, updated_at
		FROM platform.whatsapp_settings WHERE tenant_id = $1`, tenantID).
		Scan(&out.Enabled, &out.NotifyCustomers, &out.NotifySuppliers, &out.AlsoSendSMS, &out.UpdatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if out.ServiceConfigured {
		st, sErr := s.client.Status(ctx, SessionID(tenantID))
		if sErr != nil {
			out.ConnectionStatus = "unreachable"
		} else if st != nil {
			out.ConnectionStatus = st.Status
			out.Connected = st.Connected
			out.LastError = st.LastError
		}
	}
	return out, nil
}

func (s *Service) UpsertSettings(ctx context.Context, tenantID uuid.UUID, in UpsertSettingsInput) (*Settings, error) {
	cur, err := s.GetSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	enabled := cur.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	notifyCustomers := cur.NotifyCustomers
	if in.NotifyCustomers != nil {
		notifyCustomers = *in.NotifyCustomers
	}
	notifySuppliers := cur.NotifySuppliers
	if in.NotifySuppliers != nil {
		notifySuppliers = *in.NotifySuppliers
	}
	alsoSMS := cur.AlsoSendSMS
	if in.AlsoSendSMS != nil {
		alsoSMS = *in.AlsoSendSMS
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform.whatsapp_settings (
			tenant_id, enabled, notify_customers, notify_suppliers, also_send_sms, updated_at, updated_by
		) VALUES ($1,$2,$3,$4,$5, now(), $6)
		ON CONFLICT (tenant_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			notify_customers = EXCLUDED.notify_customers,
			notify_suppliers = EXCLUDED.notify_suppliers,
			also_send_sms = EXCLUDED.also_send_sms,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by`,
		tenantID, enabled, notifyCustomers, notifySuppliers, alsoSMS, in.ActorID)
	if err != nil {
		return nil, err
	}
	return s.GetSettings(ctx, tenantID)
}

// ShouldNotify reports whether outbound WhatsApp should be used for this audience,
// and whether SMS should still be queued alongside.
func (s *Service) ShouldNotify(ctx context.Context, tenantID uuid.UUID, audience string) (useWA, alsoSMS bool) {
	if s == nil || s.client == nil || !s.client.Configured() {
		return false, true
	}
	var enabled, customers, suppliers, also bool
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, notify_customers, notify_suppliers, also_send_sms
		FROM platform.whatsapp_settings WHERE tenant_id = $1`, tenantID).
		Scan(&enabled, &customers, &suppliers, &also)
	if err != nil {
		return false, true
	}
	if !enabled {
		return false, true
	}
	switch strings.ToLower(strings.TrimSpace(audience)) {
	case "customer":
		if !customers {
			return false, true
		}
	case "supplier":
		if !suppliers {
			return false, true
		}
	case "owner", "shop":
		// Shop/ops alerts (online orders, etc.): any time WhatsApp is enabled.
	default:
		return false, true
	}
	// Fall back to SMS if the Baileys session is down.
	st, err := s.client.Status(ctx, SessionID(tenantID))
	if err != nil || st == nil || !st.Connected {
		return false, true
	}
	return true, also
}

func (s *Service) SendMessage(ctx context.Context, tenantID uuid.UUID, phone, message string) error {
	if s == nil || s.client == nil || !s.client.Configured() {
		return errors.New("whatsapp service not configured")
	}
	_, err := s.client.Send(ctx, SessionID(tenantID), phone, message)
	return err
}

const (
	ActionEstimateDecide = "estimate_decide"
	ActionPartQuote      = "part_quote"
)

func digitsOnly(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Service) RememberPending(
	ctx context.Context,
	tenantID uuid.UUID,
	phone, actionType string,
	refID uuid.UUID,
	repairJobID *uuid.UUID,
	jobCode string,
	ttl time.Duration,
) error {
	if ttl <= 0 {
		ttl = 72 * time.Hour
	}
	phoneDigits := digitsOnly(phone)
	if phoneDigits == "" || refID == uuid.Nil {
		return nil
	}
	// Keep one active pending row per phone+action.
	_, _ = s.pool.Exec(ctx, `
		DELETE FROM platform.whatsapp_pending_actions
		WHERE tenant_id = $1 AND phone_digits = $2 AND action_type = $3`,
		tenantID, phoneDigits, actionType)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO platform.whatsapp_pending_actions
			(id, tenant_id, phone_digits, action_type, ref_id, repair_job_id, job_code, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.New(), tenantID, phoneDigits, actionType, refID, repairJobID, nullEmpty(jobCode), time.Now().UTC().Add(ttl))
	return err
}

type PendingAction struct {
	RefID       uuid.UUID
	RepairJobID *uuid.UUID
	JobCode     string
	ActionType  string
}

func (s *Service) LatestPending(ctx context.Context, tenantID uuid.UUID, phone, actionType string) (*PendingAction, error) {
	phoneDigits := digitsOnly(phone)
	if phoneDigits == "" {
		return nil, pgx.ErrNoRows
	}
	// Match by shared suffix (local 9 digits) so 2547… and 07… collide.
	var a PendingAction
	err := s.pool.QueryRow(ctx, `
		SELECT ref_id, repair_job_id, COALESCE(job_code, ''), action_type
		FROM platform.whatsapp_pending_actions
		WHERE tenant_id = $1
		  AND action_type = $2
		  AND expires_at > now()
		  AND (
		    phone_digits = $3
		    OR RIGHT(phone_digits, 9) = RIGHT($3, 9)
		  )
		ORDER BY created_at DESC
		LIMIT 1`, tenantID, actionType, phoneDigits).
		Scan(&a.RefID, &a.RepairJobID, &a.JobCode, &a.ActionType)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) ClearPending(ctx context.Context, tenantID, refID uuid.UUID) {
	_, _ = s.pool.Exec(ctx, `
		DELETE FROM platform.whatsapp_pending_actions WHERE tenant_id = $1 AND ref_id = $2`,
		tenantID, refID)
}

func nullEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}
