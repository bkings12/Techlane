package repair

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrVersionConflict is returned when ExpectedVersion does not match the locked row.
var ErrVersionConflict = errors.New("repair was modified by someone else — reload and try again")

// ErrEditNotAllowed is returned when the job status forbids detail corrections.
var ErrEditNotAllowed = errors.New("cannot edit details after the device has been collected")

// ErrNoDetailChanges is returned when a correction request changes nothing.
var ErrNoDetailChanges = errors.New("no changes to apply")

// UpdateRepairDetailsInput is a reasoned, permissioned correction to intake fields.
type UpdateRepairDetailsInput struct {
	TenantID        uuid.UUID
	RepairID        uuid.UUID
	ExpectedVersion int
	ProblemSummary  *string
	DeviceKind      *string
	DeviceBrand     *string
	DeviceModel     *string
	DeviceIMEI      *string
	DeviceSerial    *string
	CustomerID      *uuid.UUID
	Anonymous       *bool
	Reason          string
	ActorID         uuid.UUID
	CorrID          uuid.UUID
}

func (s *Service) UpdateRepairDetails(ctx context.Context, in UpdateRepairDetailsInput) (*RepairJob, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var (
		status        string
		branchID      uuid.UUID
		version       int
		problem       string
		customerID    *uuid.UUID
		deviceID      uuid.UUID
		devCustomerID *uuid.UUID
		devAnonymous  bool
		devKind       string
		devBrand      *string
		devModel      *string
		devIMEI       *string
		devSerial     *string
	)
	err = tx.QueryRow(ctx, `
		SELECT j.status, j.branch_id, j.version, j.problem_summary, j.customer_id, j.device_id,
		       d.customer_id, d.anonymous, d.kind, d.brand, d.model, d.imei, d.serial_number
		FROM repair.repair_jobs j
		JOIN repair.devices d ON d.id = j.device_id AND d.tenant_id = j.tenant_id
		WHERE j.tenant_id = $1 AND j.id = $2 AND j.deleted_at IS NULL
		FOR UPDATE OF j, d`, in.TenantID, in.RepairID).
		Scan(&status, &branchID, &version, &problem, &customerID, &deviceID,
			&devCustomerID, &devAnonymous, &devKind, &devBrand, &devModel, &devIMEI, &devSerial)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	if !CanEditDetails(status) {
		return nil, ErrEditNotAllowed
	}
	if in.ExpectedVersion != version {
		return nil, ErrVersionConflict
	}

	diff := map[string]any{}
	newProblem := problem
	newCustomerID := customerID
	newAnonymous := devAnonymous
	newKind := devKind
	newBrand := derefStr(devBrand)
	newModel := derefStr(devModel)
	newIMEI := derefStr(devIMEI)
	newSerial := derefStr(devSerial)
	deviceTouched := false

	if in.ProblemSummary != nil {
		next := strings.TrimSpace(*in.ProblemSummary)
		if next == "" {
			return nil, fmt.Errorf("problem_summary cannot be empty")
		}
		if next != problem {
			diff["problem_summary"] = map[string]any{"old": problem, "new": next}
			newProblem = next
		}
	}
	if in.DeviceKind != nil {
		next := strings.TrimSpace(*in.DeviceKind)
		if next == "" {
			return nil, fmt.Errorf("device_kind cannot be empty")
		}
		if next != devKind {
			diff["device_kind"] = map[string]any{"old": devKind, "new": next}
			newKind = next
			deviceTouched = true
		}
	}
	if in.DeviceBrand != nil {
		next := strings.TrimSpace(*in.DeviceBrand)
		if next != newBrand {
			diff["device_brand"] = map[string]any{"old": nullIfEmpty(newBrand), "new": nullIfEmpty(next)}
			newBrand = next
			deviceTouched = true
		}
	}
	if in.DeviceModel != nil {
		next := strings.TrimSpace(*in.DeviceModel)
		if next != newModel {
			diff["device_model"] = map[string]any{"old": nullIfEmpty(newModel), "new": nullIfEmpty(next)}
			newModel = next
			deviceTouched = true
		}
	}
	if in.DeviceIMEI != nil {
		next := strings.TrimSpace(*in.DeviceIMEI)
		if next != newIMEI {
			diff["device_imei"] = map[string]any{"old": nullIfEmpty(newIMEI), "new": nullIfEmpty(next)}
			newIMEI = next
			deviceTouched = true
		}
	}
	if in.DeviceSerial != nil {
		next := strings.TrimSpace(*in.DeviceSerial)
		if next != newSerial {
			diff["device_serial"] = map[string]any{"old": nullIfEmpty(newSerial), "new": nullIfEmpty(next)}
			newSerial = next
			deviceTouched = true
		}
	}

	switch {
	case in.Anonymous != nil && *in.Anonymous:
		if !devAnonymous {
			diff["anonymous"] = map[string]any{"old": false, "new": true}
			newAnonymous = true
			deviceTouched = true
		}
		if customerID != nil {
			diff["customer_id"] = map[string]any{"old": customerID.String(), "new": nil}
			newCustomerID = nil
		}
		if !uuidPtrEqual(devCustomerID, nil) {
			newCustomerID = nil
			deviceTouched = true
		}
	case in.CustomerID != nil:
		nextID := *in.CustomerID
		if nextID == uuid.Nil {
			return nil, fmt.Errorf("customer_id is invalid")
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM repair.customers WHERE tenant_id = $1 AND id = $2)`,
			in.TenantID, nextID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("customer not found")
		}
		if customerID == nil || *customerID != nextID {
			diff["customer_id"] = map[string]any{
				"old": uuidPtrString(customerID),
				"new": nextID.String(),
			}
			newCustomerID = &nextID
		} else {
			newCustomerID = &nextID
		}
		if devAnonymous {
			diff["anonymous"] = map[string]any{"old": true, "new": false}
			newAnonymous = false
			deviceTouched = true
		}
		if !uuidPtrEqual(devCustomerID, &nextID) {
			deviceTouched = true
		}
	case in.Anonymous != nil && !*in.Anonymous && devAnonymous:
		return nil, fmt.Errorf("customer_id is required when clearing anonymous")
	}

	if len(diff) == 0 {
		return nil, ErrNoDetailChanges
	}

	tag, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET problem_summary = $1, customer_id = $2,
		    updated_by = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $4 AND id = $5 AND version = $6`,
		newProblem, newCustomerID, in.ActorID, in.TenantID, in.RepairID, version)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrVersionConflict
	}

	if deviceTouched {
		_, err = tx.Exec(ctx, `
			UPDATE repair.devices
			SET customer_id = $1, anonymous = $2, kind = $3, brand = $4, model = $5,
			    imei = $6, serial_number = $7
			WHERE tenant_id = $8 AND id = $9`,
			newCustomerID, newAnonymous, newKind,
			nullableTrimmed(newBrand), nullableTrimmed(newModel),
			nullableTrimmed(newIMEI), nullableTrimmed(newSerial),
			in.TenantID, deviceID)
		if err != nil {
			return nil, err
		}
	}

	note := formatDetailsCorrectedNote(reason, diff)
	_, err = tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), in.TenantID, in.RepairID, "details_corrected", note, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.publish("repair.updated", in.TenantID, branchID, in.ActorID, in.CorrID, map[string]any{
		"repair_job_id": in.RepairID.String(),
		"reason":        reason,
		"changes":       diff,
	})
	return s.GetRepair(ctx, in.TenantID, in.RepairID)
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTrimmed(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func uuidPtrEqual(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func uuidPtrString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func formatDetailsCorrectedNote(reason string, diff map[string]any) string {
	fields := make([]string, 0, len(diff))
	for k := range diff {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	raw, _ := json.Marshal(diff)
	return fmt.Sprintf("Details corrected: %s. Fields: %s. Diff: %s", reason, strings.Join(fields, ", "), string(raw))
}
