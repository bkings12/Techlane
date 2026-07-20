package payments

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type TenderLine struct {
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
	Phone  string  `json:"phone,omitempty"`
}

type SplitPaymentInput struct {
	TenantID    uuid.UUID
	BranchID    *uuid.UUID
	Currency    string
	PayableType string
	PayableID   uuid.UUID
	CustomerID  *uuid.UUID
	Tenders     []TenderLine
	Phone       string
	AccountRef  string
	ActorID     uuid.UUID
	CorrID      uuid.UUID
	BalanceDue  float64
}

// CreateSplitPayment creates one payment per tender line (partial/split tender).
func (s *Service) CreateSplitPayment(ctx context.Context, in SplitPaymentInput) ([]Payment, error) {
	if len(in.Tenders) == 0 {
		return nil, fmt.Errorf("tenders required")
	}
	var sum float64
	for _, t := range in.Tenders {
		if t.Amount <= 0 {
			return nil, fmt.Errorf("each tender amount must be positive")
		}
		sum += t.Amount
	}
	if in.BalanceDue > 0 && sum > in.BalanceDue+0.009 {
		return nil, fmt.Errorf("tender total %.2f exceeds balance due %.2f", sum, in.BalanceDue)
	}
	out := make([]Payment, 0, len(in.Tenders))
	for i, t := range in.Tenders {
		phone := t.Phone
		if phone == "" {
			phone = in.Phone
		}
		corr := in.CorrID
		if corr != uuid.Nil {
			// Derive stable per-line correlation for idempotency.
			corr = uuid.NewSHA1(in.CorrID, []byte(fmt.Sprintf("%d:%s", i, t.Method)))
		}
		p, err := s.CreatePayment(ctx, CreatePaymentInput{
			TenantID: in.TenantID, BranchID: in.BranchID, Method: t.Method, Amount: t.Amount,
			Currency: in.Currency, PayableType: in.PayableType, PayableID: in.PayableID,
			Phone: phone, AccountRef: in.AccountRef, CustomerID: in.CustomerID,
			ActorID: in.ActorID, CorrID: corr, BalanceDue: in.BalanceDue,
		})
		if err != nil {
			return out, err
		}
		out = append(out, *p)
	}
	return out, nil
}
