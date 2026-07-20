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

type OperationsReport struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	PeriodDays   int                `json:"period_days"`
	Daily        []DailyMetric      `json:"daily"`
	ByTechnician []TechnicianMetric `json:"by_technician"`
	ByBranch     []BranchMetric     `json:"by_branch"`
}

func (s *Service) Summary(ctx context.Context, tenantID uuid.UUID, days int) (*Summary, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	out := &Summary{GeneratedAt: time.Now().UTC(), PeriodDays: days}

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status NOT IN ('completed', 'collected')),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'waiting_parts'),
			COUNT(*) FILTER (WHERE status = 'collected' AND updated_at >= $2)
		FROM repair.repair_jobs WHERE tenant_id = $1`, tenantID, since).
		Scan(&out.RepairsOpen, &out.RepairsReady, &out.RepairsWaitingParts, &out.RepairsCompleted)

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
			COUNT(j.id) FILTER (WHERE j.status NOT IN ('completed', 'collected')),
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
				WHERE j.tenant_id = b.tenant_id AND j.branch_id = b.id AND j.status NOT IN ('completed', 'collected')),
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
	return out, nil
}
