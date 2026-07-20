package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type AuditEvent struct {
	ID            uuid.UUID      `json:"id"`
	BranchID      *uuid.UUID     `json:"branch_id,omitempty"`
	ActorID       *uuid.UUID     `json:"actor_id,omitempty"`
	ActorName     *string        `json:"actor_name,omitempty"`
	Action        string         `json:"action"`
	EntityType    string         `json:"entity_type"`
	EntityID      *uuid.UUID     `json:"entity_id,omitempty"`
	PreviousValue map[string]any `json:"previous_value,omitempty"`
	NewValue      map[string]any `json:"new_value,omitempty"`
	Reason        *string        `json:"reason,omitempty"`
	CorrelationID *uuid.UUID     `json:"correlation_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

type RiskAlert struct {
	ID         uuid.UUID      `json:"id"`
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity"`
	Title      string         `json:"title"`
	EntityType *string        `json:"entity_type,omitempty"`
	EntityID   *uuid.UUID     `json:"entity_id,omitempty"`
	Status     string         `json:"status"`
	Details    map[string]any `json:"details,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type RecordInput struct {
	TenantID   uuid.UUID
	BranchID   *uuid.UUID
	ActorID    *uuid.UUID
	ActorRole  *string
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Previous   map[string]any
	NewValue   map[string]any
	Reason     *string
	CorrID     uuid.UUID
}

func (s *Service) AppendEvent(ctx context.Context, in RecordInput) (*AuditEvent, error) {
	id := uuid.New()
	prevJSON, _ := json.Marshal(in.Previous)
	newJSON, _ := json.Marshal(in.NewValue)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit.audit_events (id, tenant_id, branch_id, actor_id, actor_role, action, entity_type, entity_id, previous_value, new_value, reason, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		id, in.TenantID, in.BranchID, in.ActorID, in.ActorRole, in.Action, in.EntityType, in.EntityID,
		prevJSON, newJSON, in.Reason, in.CorrID)
	if err != nil {
		return nil, err
	}
	return &AuditEvent{ID: id, Action: in.Action, EntityType: in.EntityType, EntityID: in.EntityID, NewValue: in.NewValue, CreatedAt: time.Now().UTC()}, nil
}

func (s *Service) Record(ctx context.Context, tenantID uuid.UUID, actorID *uuid.UUID, action, entityType string, entityID *uuid.UUID, payload map[string]any, corrID uuid.UUID) error {
	_, err := s.AppendEvent(ctx, RecordInput{
		TenantID: tenantID, ActorID: actorID, Action: action, EntityType: entityType,
		EntityID: entityID, NewValue: payload, CorrID: corrID,
	})
	return err
}

type AuditFilter struct {
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	ActorID    *uuid.UUID
	BranchID   *uuid.UUID
	Search     string
	Limit      int
}

func (s *Service) ListEvents(ctx context.Context, tenantID uuid.UUID, f AuditFilter) ([]AuditEvent, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	q := `SELECT e.id, e.branch_id, e.actor_id, u.display_name, e.action, e.entity_type,
			e.entity_id, e.previous_value, e.new_value, e.reason, e.correlation_id, e.created_at
		FROM audit.audit_events e
		LEFT JOIN identity.users u ON u.id = e.actor_id
		WHERE e.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	add := func(clause string, value any) {
		q += fmt.Sprintf(clause, n)
		args = append(args, value)
		n++
	}
	if f.Action != "" {
		add(" AND e.action = $%d", f.Action)
	}
	if f.EntityType != "" {
		add(" AND e.entity_type = $%d", f.EntityType)
	}
	if f.EntityID != nil {
		add(" AND e.entity_id = $%d", *f.EntityID)
	}
	if f.ActorID != nil {
		add(" AND e.actor_id = $%d", *f.ActorID)
	}
	if f.BranchID != nil {
		add(" AND e.branch_id = $%d", *f.BranchID)
	}
	if f.Search != "" {
		q += fmt.Sprintf(" AND (e.action ILIKE '%%' || $%d || '%%' OR e.entity_type ILIKE '%%' || $%d || '%%' OR COALESCE(u.display_name, '') ILIKE '%%' || $%d || '%%')", n, n, n)
		args = append(args, f.Search)
		n++
	}
	q += fmt.Sprintf(" ORDER BY e.created_at DESC LIMIT $%d", n)
	args = append(args, f.Limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AuditEvent, 0)
	for rows.Next() {
		var event AuditEvent
		var previous, current []byte
		if err := rows.Scan(
			&event.ID, &event.BranchID, &event.ActorID, &event.ActorName, &event.Action,
			&event.EntityType, &event.EntityID, &previous, &current, &event.Reason,
			&event.CorrelationID, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(previous) > 0 {
			_ = json.Unmarshal(previous, &event.PreviousValue)
		}
		if len(current) > 0 {
			_ = json.Unmarshal(current, &event.NewValue)
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

func (s *Service) CreateRiskAlert(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, kind, severity, title string, entityType *string, entityID *uuid.UUID, details map[string]any) (*RiskAlert, error) {
	if entityID != nil {
		var existingID uuid.UUID
		err := s.pool.QueryRow(ctx, `
			SELECT id FROM audit.risk_alerts
			WHERE tenant_id = $1 AND kind = $2 AND entity_id = $3 AND status = 'open'
			LIMIT 1`, tenantID, kind, *entityID).Scan(&existingID)
		if err == nil {
			_, _ = s.pool.Exec(ctx, `
				UPDATE audit.risk_alerts SET title = $1, details = $2
				WHERE id = $3`, title, mustJSON(details), existingID)
			return &RiskAlert{
				ID: existingID, Kind: kind, Severity: severity, Title: title,
				EntityType: entityType, EntityID: entityID, Status: "open", Details: details, CreatedAt: time.Now().UTC(),
			}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	id := uuid.New()
	detailsJSON, _ := json.Marshal(details)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit.risk_alerts (id, tenant_id, branch_id, kind, severity, title, entity_type, entity_id, status, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'open', $9)`,
		id, tenantID, branchID, kind, severity, title, entityType, entityID, detailsJSON)
	if err != nil {
		return nil, err
	}
	return &RiskAlert{ID: id, Kind: kind, Severity: severity, Title: title, EntityType: entityType, EntityID: entityID, Status: "open", Details: details, CreatedAt: time.Now().UTC()}, nil
}

func mustJSON(v map[string]any) []byte {
	b, _ := json.Marshal(v)
	if b == nil {
		return []byte("{}")
	}
	return b
}

func (s *Service) ListRiskAlerts(ctx context.Context, tenantID uuid.UUID, status string) ([]RiskAlert, error) {
	q := `SELECT id, kind, severity, title, entity_type, entity_id, status, details, created_at
		FROM audit.risk_alerts WHERE tenant_id = $1`
	args := []any{tenantID}
	if status != "" {
		q += ` AND status = $2`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RiskAlert
	for rows.Next() {
		var a RiskAlert
		var details []byte
		if err := rows.Scan(&a.ID, &a.Kind, &a.Severity, &a.Title, &a.EntityType, &a.EntityID, &a.Status, &details, &a.CreatedAt); err != nil {
			return nil, err
		}
		if len(details) > 0 {
			_ = json.Unmarshal(details, &a.Details)
		}
		items = append(items, a)
	}
	return items, nil
}

func (s *Service) AckAlert(ctx context.Context, tenantID, alertID, resolver uuid.UUID) (*RiskAlert, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE audit.risk_alerts SET status = 'acknowledged', resolved_at = now(), resolved_by = $1
		WHERE tenant_id = $2 AND id = $3 AND status = 'open'`, resolver, tenantID, alertID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("alert not found or not open")
	}
	return s.getAlert(ctx, tenantID, alertID)
}

func (s *Service) ResolveAlert(ctx context.Context, tenantID, alertID, resolver uuid.UUID) (*RiskAlert, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE audit.risk_alerts SET status = 'resolved', resolved_at = now(), resolved_by = $1
		WHERE tenant_id = $2 AND id = $3 AND status IN ('open', 'acknowledged')`, resolver, tenantID, alertID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("alert not found or already resolved")
	}
	return s.getAlert(ctx, tenantID, alertID)
}

func (s *Service) getAlert(ctx context.Context, tenantID, alertID uuid.UUID) (*RiskAlert, error) {
	var a RiskAlert
	var details []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, kind, severity, title, entity_type, entity_id, status, details, created_at
		FROM audit.risk_alerts WHERE tenant_id = $1 AND id = $2`, tenantID, alertID).
		Scan(&a.ID, &a.Kind, &a.Severity, &a.Title, &a.EntityType, &a.EntityID, &a.Status, &details, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &RiskAlert{ID: alertID, Status: "resolved"}, nil
		}
		return nil, err
	}
	if len(details) > 0 {
		_ = json.Unmarshal(details, &a.Details)
	}
	return &a, nil
}

func (s *Service) ResolveOpenAlertsByEntity(ctx context.Context, tenantID uuid.UUID, kind string, entityID, resolver uuid.UUID) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE audit.risk_alerts
		SET status = 'resolved', resolved_at = now(), resolved_by = $1
		WHERE tenant_id = $2 AND kind = $3 AND entity_id = $4 AND status = 'open'`,
		resolver, tenantID, kind, entityID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Service) ResolveOrphanAlertsForRepair(ctx context.Context, tenantID, repairJobID, resolver uuid.UUID) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE audit.risk_alerts
		SET status = 'resolved', resolved_at = now(), resolved_by = $1
		WHERE tenant_id = $2 AND kind = 'orphan_part' AND status = 'open'
		  AND details->>'repair_job_id' = $3`,
		resolver, tenantID, repairJobID.String())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RepairCompletionAdapter clears orphan-part alerts when a job is completed.
type RepairCompletionAdapter struct {
	Svc *Service
}

func (a RepairCompletionAdapter) OnRepairCompleted(ctx context.Context, tenantID, repairJobID, actorID uuid.UUID) error {
	_, err := a.Svc.ResolveOrphanAlertsForRepair(ctx, tenantID, repairJobID, actorID)
	return err
}

func (a RepairCompletionAdapter) OnRepairStatusChanged(ctx context.Context, tenantID, repairJobID uuid.UUID, newStatus string, actorID uuid.UUID) error {
	_, _ = a.Svc.ResolveOpenAlertsByEntity(ctx, tenantID, "stuck_job", repairJobID, actorID)
	if newStatus == "collected" || newStatus == "completed" {
		_, _ = a.Svc.ResolveOpenAlertsByEntity(ctx, tenantID, "uncollected_ready", repairJobID, actorID)
	}
	return nil
}

// PaymentsRiskAdapter raises cash shortage alerts from the payments service.
type PaymentsRiskAdapter struct {
	Svc *Service
}

func (a PaymentsRiskAdapter) CreateRiskAlert(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, kind, severity, title string, entityType *string, entityID *uuid.UUID, details map[string]any) error {
	_, err := a.Svc.CreateRiskAlert(ctx, tenantID, branchID, kind, severity, title, entityType, entityID, details)
	return err
}

func (a PaymentsRiskAdapter) ResolveOpenAlertsByEntity(ctx context.Context, tenantID uuid.UUID, kind string, entityID, resolver uuid.UUID) (int64, error) {
	return a.Svc.ResolveOpenAlertsByEntity(ctx, tenantID, kind, entityID, resolver)
}
