package sales

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// PaymentTaker records payment against a sale without importing the payments package.
type PaymentTaker interface {
	TakePOSPayment(ctx context.Context, in POSPaymentInput) (*POSPaymentResult, error)
}

type POSPaymentInput struct {
	TenantID   uuid.UUID
	BranchID   uuid.UUID
	SaleID     uuid.UUID
	Method     string
	Amount     float64
	Phone      string
	AccountRef string
	ActorID    uuid.UUID
	CorrID     uuid.UUID
}

type POSPaymentResult struct {
	ID                uuid.UUID `json:"id"`
	Method            string    `json:"method"`
	Amount            float64   `json:"amount"`
	Status            string    `json:"status"`
	CheckoutRequestID string    `json:"checkout_request_id,omitempty"`
	AccountRef        string    `json:"account_reference,omitempty"`
}

type CheckoutInput struct {
	TenantID           uuid.UUID
	BranchID           uuid.UUID
	LocationID         uuid.UUID
	Items              []SaleItemInput
	Method             string
	Phone              string
	AccountRef         string
	ActorID            uuid.UUID
	CorrID             uuid.UUID
	AllowPriceOverride bool
}

type CheckoutResult struct {
	Sale      *Sale             `json:"sale"`
	Payment   *POSPaymentResult `json:"payment"`
	Completed bool              `json:"completed"`
}

func (s *Service) SetPaymentTaker(p PaymentTaker) {
	s.payments = p
}

func (s *Service) Checkout(ctx context.Context, in CheckoutInput) (*CheckoutResult, error) {
	if s.payments == nil {
		return nil, fmt.Errorf("payment service not configured")
	}
	if in.LocationID == uuid.Nil {
		return nil, fmt.Errorf("location_id required")
	}
	if in.Method == "" {
		in.Method = "cash"
	}

	sale, err := s.CreateSale(ctx, CreateSaleInput{
		TenantID: in.TenantID, BranchID: in.BranchID, Channel: "pos",
		Items: in.Items, ActorID: in.ActorID, CorrID: in.CorrID,
		AllowPriceOverride: in.AllowPriceOverride,
	})
	if err != nil {
		return nil, err
	}

	acct := strings.TrimSpace(in.AccountRef)
	if acct == "" {
		acct = "POS-" + strings.ToUpper(sale.ID.String()[:8])
	}

	pay, err := s.payments.TakePOSPayment(ctx, POSPaymentInput{
		TenantID: in.TenantID, BranchID: in.BranchID, SaleID: sale.ID,
		Method: in.Method, Amount: sale.Total, Phone: in.Phone, AccountRef: acct,
		ActorID: in.ActorID, CorrID: in.CorrID,
	})
	if err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	completed := false
	// Deduct stock when cash/bank collected or digital already allocated; STK/C2B wait for confirm.
	if in.Method == "cash" || in.Method == "bank_paybill" || in.Method == "bank_transfer" || pay.Status == "allocated" {
		sale, err = s.CompleteSale(ctx, in.TenantID, sale.ID, in.LocationID, in.ActorID, in.CorrID)
		switch {
		case err == nil:
			completed = true
		// A sale that is already out of draft has been completed by a racing
		// webhook or a retried request — the money is taken and the stock is
		// deducted, so this is the outcome we wanted, not a failure to report
		// at the counter. completeIfDraft already treats it this way.
		case strings.Contains(err.Error(), "not in draft"):
			completed = true
			if current, getErr := s.GetSale(ctx, in.TenantID, sale.ID); getErr == nil {
				sale = current
			}
		default:
			return nil, fmt.Errorf("sale complete failed: %w", err)
		}
	}

	return &CheckoutResult{Sale: sale, Payment: pay, Completed: completed}, nil
}

// CompleteAfterPayment finishes a draft POS sale after STK/bank confirmation.
func (s *Service) CompleteAfterPayment(ctx context.Context, tenantID, saleID, locationID, actorID, corrID uuid.UUID) (*Sale, error) {
	return s.CompleteSale(ctx, tenantID, saleID, locationID, actorID, corrID)
}
