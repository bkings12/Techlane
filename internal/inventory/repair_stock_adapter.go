package inventory

import (
	"context"

	"github.com/google/uuid"
)

// RepairStockAdapter lets repair deduct/restock shelf items when accessories
// are sold onto a job (same payable as the repair).
type RepairStockAdapter struct {
	Svc *Service
}

func (a RepairStockAdapter) DeductStock(
	ctx context.Context,
	tenantID, variantID, locationID uuid.UUID,
	qty int,
	reason, refType string,
	refID, actorID, corrID uuid.UUID,
) error {
	return a.Svc.ApplyMovement(ctx, tenantID, variantID, locationID, -qty, reason, refType, refID, actorID, corrID)
}

func (a RepairStockAdapter) Restock(
	ctx context.Context,
	tenantID, variantID, locationID uuid.UUID,
	qty int,
	reason, refType string,
	refID, actorID, corrID uuid.UUID,
) error {
	return a.Svc.ApplyMovement(ctx, tenantID, variantID, locationID, qty, reason, refType, refID, actorID, corrID)
}
