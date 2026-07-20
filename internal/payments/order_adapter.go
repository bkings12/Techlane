package payments

import (
	"context"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/commerce"
)

// OrderPaymentAdapter lets commerce checkout create payments against online orders.
type OrderPaymentAdapter struct {
	Svc *Service
}

func (a OrderPaymentAdapter) TakeOrderPayment(ctx context.Context, in commerce.OrderPaymentInput) (*commerce.OrderPaymentResult, error) {
	branchID := in.BranchID
	p, err := a.Svc.CreatePayment(ctx, CreatePaymentInput{
		TenantID: in.TenantID, BranchID: &branchID, Method: in.Method, Amount: in.Amount,
		Currency: "KES", PayableType: "order", PayableID: in.OrderID,
		Phone: in.Phone, AccountRef: in.AccountRef,
		ActorID: in.ActorID, CorrID: in.CorrID,
	})
	if err != nil {
		return nil, err
	}
	return &commerce.OrderPaymentResult{
		ID: p.ID, Method: p.Method, Amount: p.Amount, Status: p.Status, AccountRef: p.AccountRef,
	}, nil
}

// OrderPaidHook converts online order reservations after payment allocates.
type OrderPaidHook interface {
	OnOrderPaid(ctx context.Context, tenantID, orderID, actorID uuid.UUID) error
}

// CommercePaidAdapter wires commerce.ConfirmPaid after order payment allocates.
type CommercePaidAdapter struct {
	Svc interface {
		OnOrderPaid(ctx context.Context, tenantID, orderID, actorID uuid.UUID) error
	}
}

func (a CommercePaidAdapter) OnOrderPaid(ctx context.Context, tenantID, orderID, actorID uuid.UUID) error {
	return a.Svc.OnOrderPaid(ctx, tenantID, orderID, actorID)
}
