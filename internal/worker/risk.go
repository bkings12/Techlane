package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/audit"
	"github.com/techlane/techlane/internal/inventory"
)

type RiskScanner struct {
	pool      *pgxpool.Pool
	inventory *inventory.Service
	audit     *audit.Service
}

func NewRiskScanner(pool *pgxpool.Pool, inv *inventory.Service, aud *audit.Service) *RiskScanner {
	return &RiskScanner{pool: pool, inventory: inv, audit: aud}
}

func (w *RiskScanner) ScanAll(ctx context.Context, tenantID uuid.UUID) (int, error) {
	total := 0
	n, err := w.ScanOrphanParts(ctx, tenantID)
	if err != nil {
		return total, err
	}
	total += n
	n, err = w.ScanUnverifiedPayments(ctx, tenantID, 30*time.Minute)
	if err != nil {
		return total, err
	}
	total += n
	n, err = w.ScanStuckRepairs(ctx, tenantID, 48*time.Hour)
	if err != nil {
		return total, err
	}
	total += n
	n, err = w.ScanUncollectedReady(ctx, tenantID, 72*time.Hour)
	if err != nil {
		return total, err
	}
	total += n
	n, err = w.ExpireReservations(ctx, tenantID)
	if err != nil {
		return total, err
	}
	total += n
	return total, nil
}

func (w *RiskScanner) ScanOrphanParts(ctx context.Context, tenantID uuid.UUID) (int, error) {
	if w.inventory == nil {
		return 0, nil
	}
	issues, err := w.inventory.OrphanIssues(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, issue := range issues {
		entityType := "supplier_issue"
		entityID := issue.ID
		label := issue.JobCode
		if label == "" {
			label = issue.RepairJobID.String()
		}
		title := fmt.Sprintf("Orphan part collected for %s", label)
		_, err := w.audit.CreateRiskAlert(ctx, tenantID, nil, "orphan_part", "high", title, &entityType, &entityID, map[string]any{
			"supplier_issue_id": issue.ID.String(),
			"repair_job_id":     issue.RepairJobID.String(),
			"job_code":          issue.JobCode,
			"auth_code":         issue.AuthCode,
		})
		if err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func (w *RiskScanner) ScanUnverifiedPayments(ctx context.Context, tenantID uuid.UUID, olderThan time.Duration) (int, error) {
	if w.pool == nil {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := w.pool.Query(ctx, `
		SELECT p.id, p.method, p.amount::float8,
		       COALESCE(a.payable_type, ''), a.payable_id, p.created_at
		FROM payments.payments p
		LEFT JOIN payments.payment_allocations a ON a.payment_id = p.id
		WHERE p.tenant_id = $1
		  AND p.method IN ('mpesa_stk', 'mpesa_c2b', 'bank_paybill', 'bank_transfer')
		  AND p.status IN ('initiated', 'pending')
		  AND p.created_at < $2
		ORDER BY p.created_at ASC
		LIMIT 50`, tenantID, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	created := 0
	for rows.Next() {
		var id uuid.UUID
		var method string
		var amount float64
		var payableType string
		var payableID *uuid.UUID
		var createdAt time.Time
		if err := rows.Scan(&id, &method, &amount, &payableType, &payableID, &createdAt); err != nil {
			return created, err
		}
		entityType := "payment"
		eid := id
		title := fmt.Sprintf("Unverified %s payment KES %.0f", method, amount)
		details := map[string]any{
			"payment_id": id.String(),
			"method":     method,
			"amount":     amount,
			"age_minutes": int(time.Since(createdAt).Minutes()),
		}
		if payableType != "" {
			details["payable_type"] = payableType
		}
		if payableID != nil {
			details["payable_id"] = payableID.String()
			if payableType == "repair" {
				details["repair_job_id"] = payableID.String()
			}
		}
		if _, err := w.audit.CreateRiskAlert(ctx, tenantID, nil, "unverified_payment", "high", title, &entityType, &eid, details); err != nil {
			return created, err
		}
		created++
	}
	return created, rows.Err()
}

func (w *RiskScanner) ScanStuckRepairs(ctx context.Context, tenantID uuid.UUID, olderThan time.Duration) (int, error) {
	if w.pool == nil {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := w.pool.Query(ctx, `
		SELECT id, COALESCE(job_code,''), status, updated_at, branch_id
		FROM repair.repair_jobs
		WHERE tenant_id = $1
		  AND status IN ('waiting_parts', 'in_progress', 'diagnosed', 'intake')
		  AND updated_at < $2
		ORDER BY updated_at ASC
		LIMIT 50`, tenantID, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	created := 0
	for rows.Next() {
		var id, branchID uuid.UUID
		var jobCode, status string
		var updatedAt time.Time
		if err := rows.Scan(&id, &jobCode, &status, &updatedAt, &branchID); err != nil {
			return created, err
		}
		label := jobCode
		if label == "" {
			label = id.String()[:8]
		}
		entityType := "repair_job"
		eid := id
		bid := branchID
		title := fmt.Sprintf("Stuck repair %s (%s)", label, status)
		if _, err := w.audit.CreateRiskAlert(ctx, tenantID, &bid, "stuck_job", "medium", title, &entityType, &eid, map[string]any{
			"repair_job_id": id.String(),
			"job_code":      jobCode,
			"status":        status,
			"age_hours":     int(time.Since(updatedAt).Hours()),
		}); err != nil {
			return created, err
		}
		created++
	}
	return created, rows.Err()
}

func (w *RiskScanner) ScanUncollectedReady(ctx context.Context, tenantID uuid.UUID, olderThan time.Duration) (int, error) {
	if w.pool == nil {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := w.pool.Query(ctx, `
		SELECT id, COALESCE(job_code,''), updated_at, branch_id
		FROM repair.repair_jobs
		WHERE tenant_id = $1
		  AND status = 'completed'
		  AND updated_at < $2
		ORDER BY updated_at ASC
		LIMIT 50`, tenantID, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	created := 0
	for rows.Next() {
		var id, branchID uuid.UUID
		var jobCode string
		var updatedAt time.Time
		if err := rows.Scan(&id, &jobCode, &updatedAt, &branchID); err != nil {
			return created, err
		}
		label := jobCode
		if label == "" {
			label = id.String()[:8]
		}
		entityType := "repair_job"
		eid := id
		bid := branchID
		title := fmt.Sprintf("Ready device not collected: %s", label)
		if _, err := w.audit.CreateRiskAlert(ctx, tenantID, &bid, "uncollected_ready", "medium", title, &entityType, &eid, map[string]any{
			"repair_job_id": id.String(),
			"job_code":      jobCode,
			"age_hours":     int(time.Since(updatedAt).Hours()),
		}); err != nil {
			return created, err
		}
		created++
	}
	return created, rows.Err()
}

// ExpireReservations releases due inventory holds and marks unpaid online orders expired.
func (w *RiskScanner) ExpireReservations(ctx context.Context, tenantID uuid.UUID) (int, error) {
	if w.inventory == nil {
		return 0, nil
	}
	released, orderIDs, err := w.inventory.ExpireDueReservations(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	for _, orderID := range orderIDs {
		_, _ = w.pool.Exec(ctx, `
			UPDATE sales.orders
			SET status = 'expired', updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'pending_payment'`,
			tenantID, orderID)
		_, _ = w.pool.Exec(ctx, `
			UPDATE payments.payments p
			SET status = 'failed', updated_at = now(), version = version + 1
			FROM payments.payment_allocations a
			WHERE a.payment_id = p.id
			  AND a.payable_type = 'order' AND a.payable_id = $2
			  AND p.tenant_id = $1
			  AND p.status IN ('initiated', 'pending')`,
			tenantID, orderID)
	}
	return released, nil
}
