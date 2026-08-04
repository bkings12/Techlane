package repair

import (
	"context"

	"github.com/google/uuid"
)

// CustomerLifetimeStats aggregates a customer's commercial activity across
// repairs and retail purchases into one picture. Products sold onto a repair
// are billed as payable_type='repair' and never inserted into sales.sales,
// so summing repair-attached products alongside standalone POS sales cannot
// double-count the same purchase.
type CustomerLifetimeStats struct {
	LifetimeSpend      float64 `json:"lifetime_spend"`
	RepairsRevenue     float64 `json:"repairs_revenue"`
	RepairPartsRevenue float64 `json:"repair_parts_revenue"`
	AccessoriesRevenue float64 `json:"accessories_revenue"` // repair-attached products + standalone retail purchases
	RepairsCount       int     `json:"repairs_count"`
	RetailItemsCount   int     `json:"retail_items_count"`
	OutstandingBalance float64 `json:"outstanding_balance"`
}

// CustomerLifetimeStats computes the lifetime commercial summary for one
// customer. Reuses ListRepairs (and its already-correct line-item/legacy
// fallback money logic via applyJobMoney) rather than re-deriving totals.
func (s *Service) CustomerLifetimeStats(ctx context.Context, tenantID, customerID uuid.UUID) (*CustomerLifetimeStats, error) {
	out := &CustomerLifetimeStats{}

	repairs, err := s.ListRepairs(ctx, tenantID, ListRepairsFilter{CustomerID: &customerID})
	if err != nil {
		return nil, err
	}
	for _, r := range repairs {
		out.RepairsCount++
		labour := r.LaborAmount
		if r.ApprovedEstimateTotal != nil && *r.ApprovedEstimateTotal > 0 {
			labour = *r.ApprovedEstimateTotal
		}
		if r.hasLabourLines {
			labour = r.LabourTotal
		}
		products := r.SaleLinesTotal
		if r.hasProductLines {
			products = r.ProductsRevenue
		}
		out.RepairsRevenue += labour
		out.RepairPartsRevenue += r.PartsRevenue
		out.AccessoriesRevenue += products
		out.OutstandingBalance += r.BalanceDue
	}

	var retailRevenue float64
	var retailItems int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(si.line_total), 0)::float8, COALESCE(SUM(si.quantity), 0)
		FROM sales.sales s
		JOIN sales.sale_items si ON si.sale_id = s.id AND si.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.customer_id = $2 AND s.status = 'completed'`,
		tenantID, customerID).Scan(&retailRevenue, &retailItems); err == nil {
		out.AccessoriesRevenue += retailRevenue
		out.RetailItemsCount = retailItems
	}

	out.LifetimeSpend = out.RepairsRevenue + out.RepairPartsRevenue + out.AccessoriesRevenue
	return out, nil
}
