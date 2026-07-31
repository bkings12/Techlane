package repair

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Cost types booked against a repair job.
const (
	CostTypePartSupplier = "part_supplier"
	CostTypePartStock    = "part_stock"
	CostTypeOther        = "other"
)

// JobCost is a single cost incurred delivering a repair.
type JobCost struct {
	ID          uuid.UUID  `json:"id"`
	CostType    string     `json:"cost_type"`
	Description string     `json:"description"`
	Quantity    int        `json:"quantity"`
	UnitCost    float64    `json:"unit_cost"`
	Amount      float64    `json:"amount"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
}

// JobMargin is the money view of a single job: what the customer pays against what
// it cost to deliver. Labour is treated as revenue, not cost — technician
// commission is accounted separately.
type JobMargin struct {
	LaborAmount float64   `json:"labor_amount"`
	PartsCost   float64   `json:"parts_cost"`
	Margin      float64   `json:"margin"`
	MarginPct   *float64  `json:"margin_pct,omitempty"`
	Costs       []JobCost `json:"costs"`
	// UnpricedParts counts consumed parts we have no buying price for. Those parts
	// are real cost the margin above does not include, so the figure is a ceiling,
	// not the truth — callers must say so rather than show a flattering number.
	UnpricedParts int `json:"unpriced_parts"`
}

// BookJobCost records a cost against a job and keeps the job's rolled-up parts
// cost in step, in one transaction. Booking is idempotent per source document:
// a retried part collection cannot inflate the cost of the job.
func (s *Service) BookJobCost(
	ctx context.Context,
	tenantID, repairJobID uuid.UUID,
	costType, description string,
	quantity int,
	unitCost float64,
	refType string,
	refID, actorID uuid.UUID,
) error {
	if repairJobID == uuid.Nil {
		return nil
	}
	if quantity <= 0 {
		quantity = 1
	}
	description = strings.TrimSpace(description)
	if description == "" {
		description = "Part"
	}
	amount := unitCost * float64(quantity)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var ref any
	if refID != uuid.Nil {
		ref = refID
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO repair.job_costs
			(id, tenant_id, repair_job_id, cost_type, description, quantity, unit_cost, amount, reference_type, reference_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT DO NOTHING`,
		uuid.New(), tenantID, repairJobID, costType, description, quantity, unitCost, amount, refType, ref, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Already booked for this source document.
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs
		SET parts_cost = COALESCE(parts_cost, 0) + $1, updated_at = now()
		WHERE tenant_id = $2 AND id = $3`, amount, tenantID, repairJobID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// JobMarginFor reads the money view of a job.
func (s *Service) JobMarginFor(ctx context.Context, tenantID, repairJobID uuid.UUID) (*JobMargin, error) {
	// Heal historical gaps: parts collected before job_costs existed (or when the
	// booking hook failed) otherwise show as pure profit forever.
	_ = s.backfillSupplierCostsForJob(ctx, tenantID, repairJobID)

	m := &JobMargin{Costs: []JobCost{}}
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(labor_amount, 0)::float8, COALESCE(parts_cost, 0)::float8
		FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairJobID).Scan(&m.LaborAmount, &m.PartsCost); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, cost_type, description, quantity, unit_cost::float8, amount::float8, created_at, created_by
		FROM repair.job_costs
		WHERE tenant_id = $1 AND repair_job_id = $2
		ORDER BY created_at`, tenantID, repairJobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c JobCost
		if err := rows.Scan(&c.ID, &c.CostType, &c.Description, &c.Quantity, &c.UnitCost, &c.Amount, &c.CreatedAt, &c.CreatedBy); err != nil {
			return nil, err
		}
		if c.UnitCost <= 0 {
			m.UnpricedParts++
		}
		m.Costs = append(m.Costs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	m.Margin, m.MarginPct = ComputeMargin(m.LaborAmount, m.PartsCost)
	return m, nil
}

// backfillSupplierCostsForJob books any collected supplier issue that is missing
// a cost row for this job, then refreshes the parts_cost roll-up.
func (s *Service) backfillSupplierCostsForJob(ctx context.Context, tenantID, repairJobID uuid.UUID) error {
	rows, err := s.pool.Query(ctx, `
		SELECT si.id, si.unit_cost::float8, COALESCE(NULLIF(TRIM(pr.description), ''), 'Part'),
		       COALESCE(si.collected_by, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM inventory.supplier_issues si
		JOIN inventory.part_requests pr ON pr.id = si.part_request_id
		WHERE si.tenant_id = $1 AND si.repair_job_id = $2 AND si.status = 'collected'
		  AND NOT EXISTS (
		    SELECT 1 FROM repair.job_costs c
		    WHERE c.tenant_id = si.tenant_id
		      AND c.reference_type = 'supplier_issue'
		      AND c.reference_id = si.id
		  )`, tenantID, repairJobID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pending struct {
		issueID     uuid.UUID
		unitCost    float64
		description string
		actorID     uuid.UUID
	}
	var list []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.issueID, &p.unitCost, &p.description, &p.actorID); err != nil {
			return err
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range list {
		if err := s.BookJobCost(ctx, tenantID, repairJobID, CostTypePartSupplier, p.description, 1, p.unitCost,
			"supplier_issue", p.issueID, p.actorID); err != nil {
			return err
		}
	}
	return nil
}

// ComputeMargin returns absolute margin and margin as a percentage of revenue.
// Percentage is nil when there is no revenue to divide by — a job with costs and
// no charge is a loss, not an infinite negative percentage.
func ComputeMargin(laborAmount, partsCost float64) (float64, *float64) {
	margin := laborAmount - partsCost
	if laborAmount <= 0 {
		return margin, nil
	}
	pct := margin / laborAmount * 100
	return margin, &pct
}

// PartCostAdapter lets the inventory service book part costs onto repair jobs
// without the two packages depending on each other.
type PartCostAdapter struct {
	Svc *Service
}

func (a PartCostAdapter) BookPartCost(
	ctx context.Context,
	tenantID, repairJobID uuid.UUID,
	costType, description string,
	quantity int,
	unitCost float64,
	refType string,
	refID, actorID uuid.UUID,
) error {
	return a.Svc.BookJobCost(ctx, tenantID, repairJobID, costType, description, quantity, unitCost, refType, refID, actorID)
}
