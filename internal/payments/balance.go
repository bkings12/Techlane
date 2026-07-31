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
		  AND p.status IN ('allocated', 'confirmed', 'pending_handover', 'provisional')`,
		tenantID, payableType, payableID).Scan(&paid)
	return paid, err
}

// OutstandingResolver returns how much is still owed for a payable.
// enforceable=false means there is no priced total yet (deposits allowed).
type OutstandingResolver func(ctx context.Context, tenantID uuid.UUID, payableType string, payableID uuid.UUID) (outstanding float64, enforceable bool, err error)

func (s *Service) assertAmountWithinBalance(ctx context.Context, tenantID uuid.UUID, payableType string, payableID uuid.UUID, amount float64, balanceHint float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	outstanding := balanceHint
	enforceable := balanceHint > 0
	if s.outstanding != nil {
		if due, ok, err := s.outstanding(ctx, tenantID, payableType, payableID); err != nil {
			return err
		} else if ok {
			outstanding = due
			enforceable = true
		}
	}
	if !enforceable {
		return nil
	}
	if outstanding <= 0.009 {
		return fmt.Errorf("already fully paid — no balance due")
	}
	if amount > outstanding+0.009 {
		return fmt.Errorf("amount %.2f exceeds balance due %.2f", amount, outstanding)
	}
	return nil
}
