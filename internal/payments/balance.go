package payments

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// PayableOutstanding returns amount still due for a payable (repairs use estimate/parts totals via hook; others sum allocations against a provided total if known).
func (s *Service) AllocatedTowardPayable(ctx context.Context, tenantID uuid.UUID, payableType string, payableID uuid.UUID) (float64, error) {
	var paid float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(a.amount), 0)::float8
		FROM payments.payment_allocations a
		JOIN payments.payments p ON p.id = a.payment_id
		WHERE a.tenant_id = $1 AND a.payable_type = $2 AND a.payable_id = $3
		  AND p.status IN ('allocated', 'confirmed', 'pending_handover')`,
		tenantID, payableType, payableID).Scan(&paid)
	return paid, err
}

func (s *Service) assertAmountWithinBalance(ctx context.Context, tenantID uuid.UUID, payableType string, payableID uuid.UUID, amount float64, balanceHint float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	paid, err := s.AllocatedTowardPayable(ctx, tenantID, payableType, payableID)
	if err != nil {
		return err
	}
	// When caller supplies an explicit balance due (repair/order), enforce it.
	if balanceHint > 0 {
		if amount > balanceHint+0.009 {
			return fmt.Errorf("amount %.2f exceeds balance due %.2f", amount, balanceHint)
		}
		return nil
	}
	_ = paid
	return nil
}
