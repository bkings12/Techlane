package identity

import (
	"context"

	"github.com/google/uuid"
)

// RepairCommissionAdapter adapts identity.Service to repair.CommissionHook.
type RepairCommissionAdapter struct {
	Svc *Service
}

func (a RepairCommissionAdapter) AccrueOnRepairCompleted(
	ctx context.Context,
	tenantID, branchID, repairJobID, technicianID uuid.UUID,
	laborAmount float64,
	actorID, corrID uuid.UUID,
) error {
	_, err := a.Svc.AccrueOnRepairCompleted(ctx, tenantID, branchID, repairJobID, technicianID, laborAmount, actorID, corrID)
	return err
}
