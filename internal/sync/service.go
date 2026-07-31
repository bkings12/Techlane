package sync

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/identity"
	"github.com/techlane/techlane/internal/inventory"
	"github.com/techlane/techlane/internal/payments"
	"github.com/techlane/techlane/internal/repair"
)

type DeviceGate interface {
	AssertDeviceActive(ctx context.Context, tenantID, deviceID uuid.UUID) error
}

type Service struct {
	pool      *pgxpool.Pool
	repair    *repair.Service
	inventory *inventory.Service
	payments  *payments.Service
	devices   DeviceGate
}

func NewService(pool *pgxpool.Pool, repairSvc *repair.Service, invSvc *inventory.Service, paySvc *payments.Service, deviceGate DeviceGate) *Service {
	return &Service{pool: pool, repair: repairSvc, inventory: invSvc, payments: paySvc, devices: deviceGate}
}

type CommandInput struct {
	ActionID       uuid.UUID
	TenantID       uuid.UUID
	BranchID       *uuid.UUID
	DeviceID       *uuid.UUID
	UserID         uuid.UUID
	CommandType    string
	LocalTimestamp *time.Time
	Payload        map[string]any
	BodyHash       string
}

type CommandResult struct {
	ActionID   uuid.UUID      `json:"action_id"`
	SyncStatus string         `json:"sync_status"`
	Result     map[string]any `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type SyncCommand struct {
	ActionID       uuid.UUID      `json:"action_id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	BranchID       *uuid.UUID     `json:"branch_id,omitempty"`
	DeviceID       *uuid.UUID     `json:"device_id,omitempty"`
	UserID         uuid.UUID      `json:"user_id"`
	CommandType    string         `json:"command_type"`
	LocalTimestamp *time.Time     `json:"local_timestamp,omitempty"`
	Payload        map[string]any `json:"payload"`
	SyncStatus     string         `json:"sync_status"`
	RetryCount     int            `json:"retry_count"`
	LastError      *string        `json:"last_error,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

var ErrPayloadMismatch = errors.New("idempotency payload mismatch")
var ErrNotFound = errors.New("sync command not found")
var ErrNotRetryable = errors.New("command cannot be retried in current status")
var ErrDeviceRevoked = identity.ErrDeviceRevoked

func hashPayload(payload map[string]any) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) Submit(ctx context.Context, in CommandInput) (*CommandResult, error) {
	bodyHash, err := hashPayload(in.Payload)
	if err != nil {
		return nil, err
	}
	in.BodyHash = bodyHash

	if in.DeviceID != nil && *in.DeviceID != uuid.Nil && s.devices != nil {
		if err := s.devices.AssertDeviceActive(ctx, in.TenantID, *in.DeviceID); err != nil {
			if errors.Is(err, identity.ErrDeviceRevoked) {
				return nil, ErrDeviceRevoked
			}
			return nil, err
		}
	}

	payloadJSON, _ := json.Marshal(in.Payload)
	var claimed uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO gateway.sync_commands (action_id, tenant_id, branch_id, device_id, user_id, command_type, local_timestamp, payload, body_hash, sync_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'processing')
		ON CONFLICT (action_id) DO NOTHING
		RETURNING action_id`,
		in.ActionID, in.TenantID, in.BranchID, in.DeviceID, in.UserID, in.CommandType, in.LocalTimestamp, payloadJSON, bodyHash,
	).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.replayExisting(ctx, in.ActionID, bodyHash)
	}
	if err != nil {
		return nil, err
	}

	return s.finishDispatch(ctx, in)
}

func (s *Service) replayExisting(ctx context.Context, actionID uuid.UUID, bodyHash string) (*CommandResult, error) {
	var existingHash, syncStatus string
	var resultJSON []byte
	var lastError *string
	err := s.pool.QueryRow(ctx, `
		SELECT body_hash, sync_status, result, last_error FROM gateway.sync_commands WHERE action_id = $1`, actionID).
		Scan(&existingHash, &syncStatus, &resultJSON, &lastError)
	if err != nil {
		return nil, err
	}
	if existingHash != bodyHash {
		_, _ = s.pool.Exec(ctx, `
			UPDATE gateway.sync_commands
			SET sync_status = 'conflict', last_error = $1, updated_at = now()
			WHERE action_id = $2 AND sync_status NOT IN ('discarded')`,
			ErrPayloadMismatch.Error(), actionID)
		return nil, ErrPayloadMismatch
	}
	res := map[string]any{}
	if len(resultJSON) > 0 {
		_ = json.Unmarshal(resultJSON, &res)
	}
	out := &CommandResult{ActionID: actionID, SyncStatus: syncStatus, Result: res}
	if lastError != nil {
		out.Error = *lastError
	}
	return out, nil
}

func (s *Service) finishDispatch(ctx context.Context, in CommandInput) (*CommandResult, error) {
	result, dispatchErr := s.dispatch(ctx, in)
	status := "applied"
	errMsg := ""
	if dispatchErr != nil {
		status = "failed"
		errMsg = dispatchErr.Error()
	}
	resultJSON, _ := json.Marshal(result)
	_, err := s.pool.Exec(ctx, `
		UPDATE gateway.sync_commands SET sync_status = $1, result = $2, last_error = $3, updated_at = now()
		WHERE action_id = $4`, status, resultJSON, nullIfEmpty(errMsg), in.ActionID)
	if err != nil {
		return nil, err
	}
	out := &CommandResult{ActionID: in.ActionID, SyncStatus: status, Result: result}
	if errMsg != "" {
		out.Error = errMsg
	}
	return out, nil
}

func (s *Service) ListCommands(ctx context.Context, tenantID uuid.UUID, status string, limit int) ([]SyncCommand, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT action_id, tenant_id, branch_id, device_id, user_id, command_type, local_timestamp,
		       payload, sync_status, retry_count, last_error, result, created_at, updated_at
		FROM gateway.sync_commands
		WHERE tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if status != "" {
		if status == "needs_attention" {
			q += ` AND sync_status IN ('failed', 'conflict')`
		} else {
			q += fmt.Sprintf(` AND sync_status = $%d`, n)
			args = append(args, status)
			n++
		}
	}
	q += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT $%d`, n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SyncCommand
	for rows.Next() {
		var c SyncCommand
		var payloadRaw, resultRaw []byte
		if err := rows.Scan(
			&c.ActionID, &c.TenantID, &c.BranchID, &c.DeviceID, &c.UserID, &c.CommandType, &c.LocalTimestamp,
			&payloadRaw, &c.SyncStatus, &c.RetryCount, &c.LastError, &resultRaw, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payloadRaw, &c.Payload)
		if c.Payload == nil {
			c.Payload = map[string]any{}
		}
		if len(resultRaw) > 0 {
			_ = json.Unmarshal(resultRaw, &c.Result)
		}
		items = append(items, c)
	}
	if items == nil {
		items = []SyncCommand{}
	}
	return items, nil
}

func (s *Service) ResolveCommand(ctx context.Context, tenantID, actionID, actorID uuid.UUID, resolution string) (*CommandResult, error) {
	var cmd SyncCommand
	var payloadRaw []byte
	var bodyHash string
	err := s.pool.QueryRow(ctx, `
		SELECT action_id, tenant_id, branch_id, device_id, user_id, command_type, local_timestamp,
		       payload, body_hash, sync_status, retry_count
		FROM gateway.sync_commands
		WHERE tenant_id = $1 AND action_id = $2`, tenantID, actionID).
		Scan(&cmd.ActionID, &cmd.TenantID, &cmd.BranchID, &cmd.DeviceID, &cmd.UserID, &cmd.CommandType, &cmd.LocalTimestamp,
			&payloadRaw, &bodyHash, &cmd.SyncStatus, &cmd.RetryCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(payloadRaw, &cmd.Payload)
	if cmd.Payload == nil {
		cmd.Payload = map[string]any{}
	}

	switch resolution {
	case "discard":
		if cmd.SyncStatus != "failed" && cmd.SyncStatus != "conflict" {
			return nil, ErrNotRetryable
		}
		note := fmt.Sprintf("discarded by %s", actorID.String())
		_, err = s.pool.Exec(ctx, `
			UPDATE gateway.sync_commands
			SET sync_status = 'discarded', last_error = $1, updated_at = now()
			WHERE action_id = $2`, note, actionID)
		if err != nil {
			return nil, err
		}
		return &CommandResult{ActionID: actionID, SyncStatus: "discarded"}, nil
	case "retry":
		if cmd.SyncStatus != "failed" && cmd.SyncStatus != "conflict" {
			return nil, ErrNotRetryable
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE gateway.sync_commands
			SET sync_status = 'processing', retry_count = retry_count + 1, last_error = NULL, updated_at = now()
			WHERE action_id = $1`, actionID)
		if err != nil {
			return nil, err
		}
		return s.finishDispatch(ctx, CommandInput{
			ActionID: cmd.ActionID, TenantID: cmd.TenantID, BranchID: cmd.BranchID,
			DeviceID: cmd.DeviceID, UserID: cmd.UserID, CommandType: cmd.CommandType,
			LocalTimestamp: cmd.LocalTimestamp, Payload: cmd.Payload, BodyHash: bodyHash,
		})
	default:
		return nil, fmt.Errorf("resolution must be discard or retry")
	}
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) dispatch(ctx context.Context, in CommandInput) (map[string]any, error) {
	switch in.CommandType {
	case "repair.create_draft":
		return s.dispatchRepairCreate(ctx, in)
	case "repair.add_note":
		return s.dispatchRepairAddNote(ctx, in)
	case "repair.add_attachment":
		return s.dispatchRepairAddAttachment(ctx, in)
	case "parts.request":
		return s.dispatchPartRequest(ctx, in)
	case "payments.cash_provisional":
		return s.dispatchCashProvisional(ctx, in)
	default:
		return nil, fmt.Errorf("unknown command_type %q", in.CommandType)
	}
}

func (s *Service) dispatchRepairCreate(ctx context.Context, in CommandInput) (map[string]any, error) {
	branchID := uuidFromPayload(in.Payload, "branch_id")
	deviceID := uuidFromPayload(in.Payload, "device_id")
	if branchID == uuid.Nil || deviceID == uuid.Nil {
		return nil, fmt.Errorf("branch_id and device_id required in payload")
	}
	problem, _ := in.Payload["problem_summary"].(string)
	if problem == "" {
		problem = "Offline draft repair"
	}
	var customerID *uuid.UUID
	if cid := uuidFromPayload(in.Payload, "customer_id"); cid != uuid.Nil {
		customerID = &cid
	}
	var techID *uuid.UUID
	if tid := uuidFromPayload(in.Payload, "technician_id"); tid != uuid.Nil {
		techID = &tid
	}
	var clientID *uuid.UUID
	if id := uuidFromPayload(in.Payload, "repair_job_id"); id != uuid.Nil {
		clientID = &id
	}
	job, err := s.repair.CreateRepair(ctx, repair.CreateRepairInput{
		BranchID: branchID, CustomerID: customerID, DeviceID: deviceID,
		ProblemSummary: problem, TechnicianID: techID,
		ActorID: in.UserID, TenantID: in.TenantID, ClientID: clientID, CorrID: in.ActionID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"repair_job_id": job.ID.String(), "status": job.Status}, nil
}

func (s *Service) dispatchRepairAddNote(ctx context.Context, in CommandInput) (map[string]any, error) {
	repairID := uuidFromPayload(in.Payload, "repair_job_id")
	note, _ := in.Payload["note"].(string)
	if repairID == uuid.Nil || strings.TrimSpace(note) == "" {
		return nil, fmt.Errorf("repair_job_id and note required in payload")
	}
	var clientID *uuid.UUID
	if id := uuidFromPayload(in.Payload, "note_id"); id != uuid.Nil {
		clientID = &id
	}
	n, err := s.repair.AddNote(ctx, in.TenantID, repairID, strings.TrimSpace(note), in.UserID, in.ActionID, clientID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"note_id": n.ID.String()}, nil
}

func (s *Service) dispatchRepairAddAttachment(ctx context.Context, in CommandInput) (map[string]any, error) {
	repairID := uuidFromPayload(in.Payload, "repair_job_id")
	fileName, _ := in.Payload["file_name"].(string)
	contentType, _ := in.Payload["content_type"].(string)
	dataB64, _ := in.Payload["content_base64"].(string)
	if repairID == uuid.Nil || strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || dataB64 == "" {
		return nil, fmt.Errorf("repair_job_id, file_name, content_type, and content_base64 required")
	}
	content, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil || len(content) == 0 {
		return nil, fmt.Errorf("invalid content_base64")
	}
	var clientID *uuid.UUID
	if id := uuidFromPayload(in.Payload, "attachment_id"); id != uuid.Nil {
		clientID = &id
	}
	att, err := s.repair.AddAttachment(ctx, in.TenantID, repairID, fileName, contentType, content, in.UserID, in.ActionID, clientID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"attachment_id": att.ID.String()}, nil
}

func (s *Service) dispatchPartRequest(ctx context.Context, in CommandInput) (map[string]any, error) {
	if s.inventory == nil {
		return nil, fmt.Errorf("inventory service not configured")
	}
	branchID := uuidFromPayload(in.Payload, "branch_id")
	repairID := uuidFromPayload(in.Payload, "repair_job_id")
	desc, _ := in.Payload["description"].(string)
	if branchID == uuid.Nil || repairID == uuid.Nil || strings.TrimSpace(desc) == "" {
		return nil, fmt.Errorf("branch_id, repair_job_id, and description required in payload")
	}
	qty := intFromPayload(in.Payload, "quantity", 1)
	var variantID *uuid.UUID
	if vid := uuidFromPayload(in.Payload, "variant_id"); vid != uuid.Nil {
		variantID = &vid
	}
	var clientID *uuid.UUID
	if id := uuidFromPayload(in.Payload, "part_request_id"); id != uuid.Nil {
		clientID = &id
	}
	var supplierID *uuid.UUID
	if sid := uuidFromPayload(in.Payload, "supplier_id"); sid != uuid.Nil {
		supplierID = &sid
	}
	pr, err := s.inventory.CreatePartRequest(ctx, in.TenantID, branchID, repairID, variantID, desc, qty, supplierID, in.UserID, in.ActionID, clientID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"part_request_id": pr.ID.String(), "status": pr.Status}, nil
}

func (s *Service) dispatchCashProvisional(ctx context.Context, in CommandInput) (map[string]any, error) {
	if s.payments == nil {
		return nil, fmt.Errorf("payments service not configured")
	}
	branchID := uuidFromPayload(in.Payload, "branch_id")
	payableID := uuidFromPayload(in.Payload, "payable_id")
	payableType, _ := in.Payload["payable_type"].(string)
	if payableType == "" {
		payableType = "repair"
	}
	amount := floatFromPayload(in.Payload, "amount")
	if branchID == uuid.Nil || payableID == uuid.Nil || amount <= 0 {
		return nil, fmt.Errorf("branch_id, payable_id, and positive amount required in payload")
	}
	var clientID *uuid.UUID
	if id := uuidFromPayload(in.Payload, "payment_id"); id != uuid.Nil {
		clientID = &id
	}
	bid := branchID
	pay, err := s.payments.CreatePayment(ctx, payments.CreatePaymentInput{
		TenantID: in.TenantID, BranchID: &bid, Method: "cash", Amount: amount,
		PayableType: payableType, PayableID: payableID, ActorID: in.UserID,
		ClientID: clientID, CorrID: in.ActionID, BodyHash: in.BodyHash,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"payment_id": pay.ID.String(), "status": pay.Status}, nil
}

func uuidFromPayload(p map[string]any, key string) uuid.UUID {
	v, ok := p[key]
	if !ok {
		return uuid.Nil
	}
	switch t := v.(type) {
	case string:
		id, _ := uuid.Parse(t)
		return id
	default:
		return uuid.Nil
	}
}

func intFromPayload(p map[string]any, key string, def int) int {
	v, ok := p[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return def
	}
}

func floatFromPayload(p map[string]any, key string) float64 {
	v, ok := p[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}
