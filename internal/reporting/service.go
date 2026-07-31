package reporting

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Summary struct {
	GeneratedAt time.Time `json:"generated_at"`
	PeriodDays  int       `json:"period_days"`

	RepairsOpen         int `json:"repairs_open"`
	RepairsReady        int `json:"repairs_ready"`
	RepairsCompleted    int `json:"repairs_completed_period"`
	RepairsWaitingParts int `json:"repairs_waiting_parts"`
	RepairsClosedPeriod int `json:"repairs_closed_period"`
	RepairsOverdue      int `json:"repairs_overdue"`

	// Money still sitting in the workshop / waiting at the counter.
	RepairsPipelineValue    float64 `json:"repairs_pipeline_value"`
	RepairsCollectibleValue float64 `json:"repairs_collectible_value"`
	RepairsUnpriced         int     `json:"repairs_unpriced"`

	PaymentsAllocatedPeriod float64 `json:"payments_allocated_period"`
	PaymentsCashProvisional float64 `json:"payments_cash_provisional"`
	PaymentsSTKPending      float64 `json:"payments_stk_pending"`
	SalesCompletedPeriod    float64 `json:"sales_completed_period"`
	SalesCountPeriod        int     `json:"sales_count_period"`

	HandoversOpen             int     `json:"handovers_open"`
	ShortageAmountPeriod      float64 `json:"shortage_amount_period"`
	SupplierCreditOutstanding float64 `json:"supplier_credit_outstanding"`

	RiskOpenTotal     int `json:"risk_open_total"`
	RiskOrphanParts   int `json:"risk_orphan_parts"`
	RiskCashShortage  int `json:"risk_cash_shortage"`
	RiskUnverifiedPay int `json:"risk_unverified_payment"`
	RiskStuckJobs     int `json:"risk_stuck_jobs"`
}

type DailyMetric struct {
	Date              string  `json:"date"`
	PaymentsAllocated float64 `json:"payments_allocated"`
	SalesCompleted    float64 `json:"sales_completed"`
	RepairsCompleted  int     `json:"repairs_completed"`
}

type TechnicianMetric struct {
	TechnicianID uuid.UUID `json:"technician_id"`
	Name         string    `json:"name"`
	OpenJobs     int       `json:"open_jobs"`
	Completed    int       `json:"completed_period"`
	LaborAmount  float64   `json:"labor_amount_period"`
}

type BranchMetric struct {
	BranchID   uuid.UUID `json:"branch_id"`
	Name       string    `json:"name"`
	OpenJobs   int       `json:"open_jobs"`
	Completed  int       `json:"completed_period"`
	SalesTotal float64   `json:"sales_total_period"`
}

// ClosureMetric answers "why are we losing jobs?" — grouped by the structured
// reason code recorded when a job is cancelled or written off.
// RepairProfitability rolls up what we charged for labor against the parts we
// consumed on jobs finished in the period, so an owner can see whether repair
// work is actually earning after part costs.
type RepairProfitability struct {
	Jobs           int      `json:"jobs"`
	LaborRevenue   float64  `json:"labor_revenue"`
	PartsCost      float64  `json:"parts_cost"`
	Margin         float64  `json:"margin"`
	MarginPct      *float64 `json:"margin_pct,omitempty"`
	LossMakingJobs int      `json:"loss_making_jobs"`
	// Jobs that consumed a part with no buying price on record. Their true cost is
	// higher than what is reported here, so margin is an upper bound.
	JobsWithUnpricedParts int `json:"jobs_with_unpriced_parts"`
}

type ClosureMetric struct {
	Status         string  `json:"status"`
	Reason         string  `json:"reason"`
	Count          int     `json:"count"`
	LostLaborValue float64 `json:"lost_labor_value"`
}

type OperationsReport struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	PeriodDays   int                `json:"period_days"`
	Daily        []DailyMetric      `json:"daily"`
	ByTechnician []TechnicianMetric `json:"by_technician"`
	ByBranch     []BranchMetric     `json:"by_branch"`
	Closures     []ClosureMetric    `json:"closures"`

	Profitability RepairProfitability `json:"repair_profitability"`
}

func (s *Service) Summary(ctx context.Context, tenantID uuid.UUID, days int) (*Summary, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	out := &Summary{GeneratedAt: time.Now().UTC(), PeriodDays: days}

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status NOT IN ('completed', 'collected', 'cancelled', 'unrepairable')),
			COUNT(*) FILTER (WHERE status = 'ready_for_pickup'),
			COUNT(*) FILTER (WHERE status = 'waiting_parts'),
			COUNT(*) FILTER (WHERE status = 'collected' AND updated_at >= $2),
			COUNT(*) FILTER (WHERE status IN ('cancelled', 'unrepairable') AND closed_at >= $2),
			COUNT(*) FILTER (WHERE promised_by < now() AND status NOT IN ('ready_for_pickup', 'completed', 'collected', 'cancelled', 'unrepairable'))
		FROM repair.repair_jobs WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID, since).
		Scan(&out.RepairsOpen, &out.RepairsReady, &out.RepairsWaitingParts, &out.RepairsCompleted, &out.RepairsClosedPeriod, &out.RepairsOverdue)

	_ = s.pool.QueryRow(ctx, `
		WITH open_jobs AS (
			SELECT j.id, j.status,
				COALESCE(j.labor_amount, 0)::float8 AS labor,
				j.authorized_amount::float8 AS authorized,
				COALESCE((
					SELECT SUM(sl.line_total)::float8 FROM repair.job_sale_lines sl
					WHERE sl.tenant_id = j.tenant_id AND sl.repair_job_id = j.id
				), 0) AS sales,
				(
					SELECT (e.labor_amount + e.parts_amount)::float8
					FROM repair.repair_estimates e
					WHERE e.tenant_id = j.tenant_id AND e.repair_job_id = j.id AND e.status = 'approved'
					ORDER BY e.decided_at DESC NULLS LAST, e.created_at DESC
					LIMIT 1
				) AS approved,
				(
					SELECT (e.labor_amount + e.parts_amount)::float8
					FROM repair.repair_estimates e
					WHERE e.tenant_id = j.tenant_id AND e.repair_job_id = j.id AND e.status = 'pending'
					ORDER BY e.created_at DESC
					LIMIT 1
				) AS pending,
				COALESCE((
					SELECT SUM(a.amount)::float8
					FROM payments.payment_allocations a
					JOIN payments.payments p ON p.id = a.payment_id
					WHERE a.tenant_id = j.tenant_id
					  AND a.payable_type = 'repair'
					  AND a.payable_id = j.id
					  AND p.status IN ('allocated', 'confirmed', 'pending_handover', 'provisional')
				), 0) AS paid
			FROM repair.repair_jobs j
			WHERE j.tenant_id = $1
			  AND j.deleted_at IS NULL
			  AND j.status NOT IN ('collected', 'cancelled', 'unrepairable')
		),
		valued AS (
			SELECT *,
				COALESCE(approved, NULLIF(labor, 0), authorized, pending, 0) + sales AS quoted,
				GREATEST(0, COALESCE(approved, labor, 0) + sales - paid) AS balance
			FROM open_jobs
		)
		SELECT
			COALESCE(SUM(quoted), 0)::float8,
			COALESCE(SUM(balance) FILTER (WHERE status IN ('ready_for_pickup', 'completed')), 0)::float8,
			COUNT(*) FILTER (WHERE quoted <= 0)
		FROM valued`, tenantID).
		Scan(&out.RepairsPipelineValue, &out.RepairsCollectibleValue, &out.RepairsUnpriced)

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE status = 'allocated' AND created_at >= $2), 0)::float8,
			COALESCE(SUM(amount) FILTER (WHERE method = 'cash' AND status = 'pending_handover'), 0)::float8,
			COALESCE(SUM(amount) FILTER (WHERE method = 'mpesa_stk' AND status IN ('initiated', 'pending')), 0)::float8
		FROM payments.payments WHERE tenant_id = $1`, tenantID, since).
		Scan(&out.PaymentsAllocatedPeriod, &out.PaymentsCashProvisional, &out.PaymentsSTKPending)

	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total), 0)::float8, COUNT(*)
		FROM sales.sales
		WHERE tenant_id = $1 AND status = 'completed' AND created_at >= $2`, tenantID, since).
		Scan(&out.SalesCompletedPeriod, &out.SalesCountPeriod)

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'requested'),
			COALESCE(SUM(shortage_amount) FILTER (WHERE status = 'confirmed' AND confirmed_at >= $2), 0)::float8
		FROM payments.cash_handovers WHERE tenant_id = $1`, tenantID, since).
		Scan(&out.HandoversOpen, &out.ShortageAmountPeriod)

	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE
			WHEN entry_type = 'issue' THEN amount
			WHEN entry_type IN ('settlement', 'payment', 'adjustment') THEN -amount
			ELSE 0 END), 0)::float8
		FROM inventory.supplier_credit_entries WHERE tenant_id = $1`, tenantID).
		Scan(&out.SupplierCreditOutstanding)

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE kind = 'orphan_part'),
			COUNT(*) FILTER (WHERE kind = 'cash_shortage'),
			COUNT(*) FILTER (WHERE kind = 'unverified_payment'),
			COUNT(*) FILTER (WHERE kind IN ('stuck_job', 'uncollected_ready'))
		FROM audit.risk_alerts WHERE tenant_id = $1 AND status = 'open'`, tenantID).
		Scan(&out.RiskOpenTotal, &out.RiskOrphanParts, &out.RiskCashShortage, &out.RiskUnverifiedPay, &out.RiskStuckJobs)

	return out, nil
}

func (s *Service) Operations(ctx context.Context, tenantID uuid.UUID, days int) (*OperationsReport, error) {
	if days <= 0 || days > 90 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -(days - 1))
	out := &OperationsReport{
		GeneratedAt:  time.Now().UTC(),
		PeriodDays:   days,
		Daily:        make([]DailyMetric, 0),
		ByTechnician: make([]TechnicianMetric, 0),
		ByBranch:     make([]BranchMetric, 0),
		Closures:     make([]ClosureMetric, 0),
	}

	rows, err := s.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series($2::date, CURRENT_DATE, interval '1 day')::date AS day
		)
		SELECT d.day::text,
			COALESCE((SELECT SUM(p.amount)::float8 FROM payments.payments p
				WHERE p.tenant_id = $1 AND p.status = 'allocated' AND p.created_at::date = d.day), 0),
			COALESCE((SELECT SUM(sa.total)::float8 FROM sales.sales sa
				WHERE sa.tenant_id = $1 AND sa.status = 'completed' AND sa.created_at::date = d.day), 0),
			(SELECT COUNT(*) FROM repair.repair_jobs j
				WHERE j.tenant_id = $1 AND j.status IN ('completed', 'collected') AND j.updated_at::date = d.day)
		FROM days d ORDER BY d.day`, tenantID, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item DailyMetric
		if err := rows.Scan(&item.Date, &item.PaymentsAllocated, &item.SalesCompleted, &item.RepairsCompleted); err != nil {
			rows.Close()
			return nil, err
		}
		out.Daily = append(out.Daily, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
		SELECT u.id, u.display_name,
			COUNT(j.id) FILTER (WHERE j.status NOT IN ('completed', 'collected', 'cancelled', 'unrepairable')),
			COUNT(j.id) FILTER (WHERE j.status IN ('completed', 'collected') AND j.updated_at >= $2),
			COALESCE(SUM(j.labor_amount) FILTER (WHERE j.status IN ('completed', 'collected') AND j.updated_at >= $2), 0)::float8
		FROM identity.users u
		JOIN identity.employee_profiles ep ON ep.user_id = u.id AND ep.is_technician
		LEFT JOIN repair.repair_jobs j ON j.technician_id = u.id AND j.tenant_id = u.tenant_id
		WHERE u.tenant_id = $1
		GROUP BY u.id, u.display_name
		ORDER BY u.display_name`, tenantID, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item TechnicianMetric
		if err := rows.Scan(&item.TechnicianID, &item.Name, &item.OpenJobs, &item.Completed, &item.LaborAmount); err != nil {
			rows.Close()
			return nil, err
		}
		out.ByTechnician = append(out.ByTechnician, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
		SELECT b.id, b.name,
			(SELECT COUNT(*) FROM repair.repair_jobs j
				WHERE j.tenant_id = b.tenant_id AND j.branch_id = b.id
				AND j.status NOT IN ('completed', 'collected', 'cancelled', 'unrepairable')),
			(SELECT COUNT(*) FROM repair.repair_jobs j
				WHERE j.tenant_id = b.tenant_id AND j.branch_id = b.id
				AND j.status IN ('completed', 'collected') AND j.updated_at >= $2),
			COALESCE((SELECT SUM(sa.total)::float8 FROM sales.sales sa
				WHERE sa.tenant_id = b.tenant_id AND sa.branch_id = b.id
				AND sa.status = 'completed' AND sa.created_at >= $2), 0)
		FROM identity.branches b WHERE b.tenant_id = $1 ORDER BY b.name`, tenantID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item BranchMetric
		if err := rows.Scan(&item.BranchID, &item.Name, &item.OpenJobs, &item.Completed, &item.SalesTotal); err != nil {
			return nil, err
		}
		out.ByBranch = append(out.ByBranch, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	closures, err := s.pool.Query(ctx, `
		SELECT status, closure_reason, COUNT(*),
		       COALESCE(SUM(labor_amount), 0)::float8
		FROM repair.repair_jobs
		WHERE tenant_id = $1 AND closure_reason IS NOT NULL AND closed_at >= $2
		GROUP BY status, closure_reason
		ORDER BY COUNT(*) DESC`, tenantID, since)
	if err != nil {
		return nil, err
	}
	defer closures.Close()
	for closures.Next() {
		var item ClosureMetric
		if err := closures.Scan(&item.Status, &item.Reason, &item.Count, &item.LostLaborValue); err != nil {
			return nil, err
		}
		out.Closures = append(out.Closures, item)
	}
	if err := closures.Err(); err != nil {
		return nil, err
	}
	closures.Close()

	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(j.labor_amount), 0)::float8,
		       COALESCE(SUM(j.parts_cost), 0)::float8,
		       COUNT(*) FILTER (WHERE COALESCE(j.parts_cost, 0) > j.labor_amount),
		       COUNT(*) FILTER (WHERE EXISTS (
		           SELECT 1 FROM repair.job_costs c
		           WHERE c.repair_job_id = j.id AND c.unit_cost <= 0
		       ))
		FROM repair.repair_jobs j
		WHERE j.tenant_id = $1 AND j.status IN ('completed', 'collected') AND j.updated_at >= $2`, tenantID, since).
		Scan(&out.Profitability.Jobs, &out.Profitability.LaborRevenue,
			&out.Profitability.PartsCost, &out.Profitability.LossMakingJobs,
			&out.Profitability.JobsWithUnpricedParts)
	out.Profitability.Margin = out.Profitability.LaborRevenue - out.Profitability.PartsCost
	if out.Profitability.LaborRevenue > 0 {
		pct := out.Profitability.Margin / out.Profitability.LaborRevenue * 100
		out.Profitability.MarginPct = &pct
	}

	return out, nil
}
