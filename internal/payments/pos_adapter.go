package payments

import (
	"context"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/sales"
)

// SalePaymentAdapter lets the sales POS checkout create payments.
type SalePaymentAdapter struct {
	Svc *Service
}

func (a SalePaymentAdapter) TakePOSPayment(ctx context.Context, in sales.POSPaymentInput) (*sales.POSPaymentResult, error) {
	branchID := in.BranchID
	acct := in.AccountRef
	if acct == "" {
		acct = "POS"
	}
	p, err := a.Svc.CreatePayment(ctx, CreatePaymentInput{
		TenantID: in.TenantID, BranchID: &branchID, Method: in.Method, Amount: in.Amount,
		Currency: "KES", PayableType: "sale", PayableID: in.SaleID,
		Phone: in.Phone, AccountRef: acct,
		ActorID: in.ActorID, CorrID: in.CorrID,
	})
	if err != nil {
		return nil, err
	}
	return &sales.POSPaymentResult{
		ID: p.ID, Method: p.Method, Amount: p.Amount, Status: p.Status,
		CheckoutRequestID: p.CheckoutRequestID, AccountRef: p.AccountRef,
	}, nil
}

// QuickSaleCreator lets an unmatched C2B payment be resolved by creating (and
// completing — deducting stock) a one-line sale for the product/quantity the
// owner picks. The money already arrived; this just explains what it was for.
type QuickSaleCreator interface {
	CreateQuickSale(ctx context.Context, in QuickSaleInput) (*sales.Sale, error)
}

type QuickSaleInput struct {
	TenantID   uuid.UUID
	BranchID   uuid.UUID
	LocationID uuid.UUID
	VariantID  uuid.UUID
	Quantity   int
	ActorID    uuid.UUID
	CorrID     uuid.UUID
}

// SalesQuickSaleAdapter lets unmatched-payment matching create a sale on the spot.
type SalesQuickSaleAdapter struct {
	Svc *sales.Service
}

func (a SalesQuickSaleAdapter) CreateQuickSale(ctx context.Context, in QuickSaleInput) (*sales.Sale, error) {
	sale, err := a.Svc.CreateSale(ctx, sales.CreateSaleInput{
		TenantID: in.TenantID, BranchID: in.BranchID, Channel: "pos",
		Items:    []sales.SaleItemInput{{VariantID: in.VariantID, Quantity: in.Quantity}},
		ActorID:  in.ActorID, CorrID: in.CorrID,
	})
	if err != nil {
		return nil, err
	}
	return a.Svc.CompleteSale(ctx, in.TenantID, sale.ID, in.LocationID, in.ActorID, in.CorrID)
}

// SalesPaidAdapter completes a draft POS sale after its STK/C2B payment allocates.
type SalesPaidAdapter struct {
	Svc interface {
		CompletePaidDraftSale(ctx context.Context, tenantID, saleID, actorID uuid.UUID) error
	}
}

func (a SalesPaidAdapter) OnSalePaymentSettled(ctx context.Context, tenantID, saleID, actorID uuid.UUID) error {
	return a.Svc.CompletePaidDraftSale(ctx, tenantID, saleID, actorID)
}
