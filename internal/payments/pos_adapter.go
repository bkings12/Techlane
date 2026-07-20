package payments

import (
	"context"

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
