package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type C2BPayload struct {
	TransID            string
	TransType          string
	TransTime          string
	Amount             float64
	BusinessShortCode  string
	BillRefNumber      string
	InvoiceNumber      string
	MSISDN             string
	FirstName          string
	MiddleName         string
	LastName           string
	OrgAccountBalance  string
	ThirdPartyTransID  string
	Raw                json.RawMessage
}

type C2BTransaction struct {
	ID              uuid.UUID  `json:"id"`
	PaymentID       *uuid.UUID `json:"payment_id,omitempty"`
	TransID         string     `json:"trans_id,omitempty"`
	Amount          float64    `json:"amount"`
	BusinessShort   string     `json:"business_shortcode,omitempty"`
	BillRefNumber   string     `json:"bill_ref_number,omitempty"`
	MSISDN          string     `json:"msisdn,omitempty"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ValidateC2B decides whether Safaricom should accept the payment.
// Unknown bill refs are still accepted so cash is not rejected at the till —
// confirmation stores unmatched rows for staff to allocate.
func (s *Service) ValidateC2B(ctx context.Context, shortcode, billRef string) (accepted bool, desc string) {
	shortcode = strings.TrimSpace(shortcode)
	if shortcode == "" {
		return true, "Accepted"
	}
	var tenantID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT tenant_id FROM payments.provider_settings
		WHERE mpesa_shortcode = $1 LIMIT 1`, shortcode).Scan(&tenantID)
	if err != nil {
		// Unknown shortcode — still accept; confirmation may no-op.
		return true, "Accepted"
	}
	_ = tenantID
	_ = billRef
	return true, "Accepted"
}

// ProcessC2BConfirmation is idempotent on TransID. Matches pending C2B payments by bill ref,
// confirms when amounts align, otherwise records unmatched / amount_mismatch and raises risk.
func (s *Service) ProcessC2BConfirmation(ctx context.Context, p C2BPayload) error {
	if strings.TrimSpace(p.TransID) == "" {
		return fmt.Errorf("TransID required")
	}

	var tenantID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT tenant_id FROM payments.provider_settings
		WHERE mpesa_shortcode = $1 LIMIT 1`, strings.TrimSpace(p.BusinessShortCode)).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No tenant for shortcode — ignore quietly (Safaricom still gets Accepted).
			return nil
		}
		return err
	}

	// Idempotency: already processed this TransID.
	var existingStatus string
	var existingPayment *uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT status, payment_id FROM payments.mpesa_c2b_transactions
		WHERE tenant_id = $1 AND trans_id = $2 LIMIT 1`, tenantID, p.TransID).
		Scan(&existingStatus, &existingPayment)
	if err == nil {
		if existingStatus == "confirmed" || existingStatus == "unmatched" || existingStatus == "amount_mismatch" {
			return nil
		}
		if existingPayment != nil && *existingPayment != uuid.Nil {
			_, _ = s.ConfirmMpesaWebhook(ctx, tenantID, *existingPayment, p.TransID)
			return nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	billRef := strings.TrimSpace(p.BillRefNumber)
	var paymentID uuid.UUID
	var expected float64
	matchErr := s.pool.QueryRow(ctx, `
		SELECT c.payment_id, p.amount::float8
		FROM payments.mpesa_c2b_transactions c
		JOIN payments.payments p ON p.id = c.payment_id
		WHERE c.tenant_id = $1
		  AND c.status IN ('pending', 'awaiting', 'initiated')
		  AND c.payment_id IS NOT NULL
		  AND c.bill_ref_number = $2
		  AND p.status IN ('initiated', 'pending')
		ORDER BY c.created_at DESC
		LIMIT 1`, tenantID, billRef).Scan(&paymentID, &expected)

	raw := p.Raw
	if len(raw) == 0 {
		raw, _ = json.Marshal(map[string]any{
			"TransID": p.TransID, "BillRefNumber": billRef, "TransAmount": p.Amount,
		})
	}

	if matchErr != nil {
		if !errors.Is(matchErr, pgx.ErrNoRows) {
			return matchErr
		}
		return s.recordUnmatchedC2B(ctx, tenantID, p, raw)
	}

	if math.Abs(expected-p.Amount) > 0.5 {
		return s.recordAmountMismatchC2B(ctx, tenantID, paymentID, expected, p, raw)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE payments.mpesa_c2b_transactions SET
			trans_id = $1, trans_type = $2, trans_time = $3, amount = $4, business_shortcode = $5,
			msisdn = $6, first_name = $7, middle_name = $8, last_name = $9,
			org_account_balance = $10, third_party_trans_id = $11, raw_payload = $12,
			invoice_number = $13, updated_at = now()
		WHERE payment_id = $14`,
		p.TransID, nullIfEmpty(p.TransType), nullIfEmpty(p.TransTime), p.Amount, nullIfEmpty(p.BusinessShortCode),
		nullIfEmpty(p.MSISDN), nullIfEmpty(p.FirstName), nullIfEmpty(p.MiddleName), nullIfEmpty(p.LastName),
		nullIfEmpty(p.OrgAccountBalance), nullIfEmpty(p.ThirdPartyTransID), raw,
		nullIfEmpty(p.InvoiceNumber), paymentID,
	)
	if err != nil {
		return err
	}
	_, err = s.ConfirmMpesaWebhook(ctx, tenantID, paymentID, p.TransID)
	return err
}

func (s *Service) recordUnmatchedC2B(ctx context.Context, tenantID uuid.UUID, p C2BPayload, raw json.RawMessage) error {
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO payments.mpesa_c2b_transactions (
			id, tenant_id, payment_id, trans_id, trans_type, trans_time, amount, business_shortcode,
			bill_ref_number, invoice_number, msisdn, first_name, middle_name, last_name,
			org_account_balance, third_party_trans_id, raw_payload, status
		) VALUES (
			$1, $2, NULL, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 'unmatched'
		)`,
		id, tenantID, p.TransID, nullIfEmpty(p.TransType), nullIfEmpty(p.TransTime), p.Amount,
		nullIfEmpty(p.BusinessShortCode), nullIfEmpty(p.BillRefNumber), nullIfEmpty(p.InvoiceNumber),
		nullIfEmpty(p.MSISDN), nullIfEmpty(p.FirstName), nullIfEmpty(p.MiddleName), nullIfEmpty(p.LastName),
		nullIfEmpty(p.OrgAccountBalance), nullIfEmpty(p.ThirdPartyTransID), raw,
	)
	if err != nil {
		// Race on unique TransID — treat as already processed.
		if strings.Contains(err.Error(), "idx_mpesa_c2b_trans_id") || strings.Contains(err.Error(), "duplicate key") {
			return nil
		}
		return err
	}
	if s.riskHook != nil {
		et := "mpesa_c2b"
		eid := id
		title := fmt.Sprintf("Unmatched C2B KES %.0f · ref %s", p.Amount, strings.TrimSpace(p.BillRefNumber))
		_ = s.riskHook.CreateRiskAlert(ctx, tenantID, nil, "unmatched_c2b", "high", title, &et, &eid, map[string]any{
			"trans_id":         p.TransID,
			"bill_ref_number":  p.BillRefNumber,
			"amount":           p.Amount,
			"msisdn":           p.MSISDN,
			"business_shortcode": p.BusinessShortCode,
		})
	}
	return nil
}

func (s *Service) recordAmountMismatchC2B(ctx context.Context, tenantID, paymentID uuid.UUID, expected float64, p C2BPayload, raw json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE payments.mpesa_c2b_transactions SET
			trans_id = $1, trans_type = $2, trans_time = $3, amount = $4, business_shortcode = $5,
			msisdn = $6, first_name = $7, middle_name = $8, last_name = $9,
			org_account_balance = $10, third_party_trans_id = $11, raw_payload = $12,
			status = 'amount_mismatch', updated_at = now()
		WHERE payment_id = $13`,
		p.TransID, nullIfEmpty(p.TransType), nullIfEmpty(p.TransTime), p.Amount, nullIfEmpty(p.BusinessShortCode),
		nullIfEmpty(p.MSISDN), nullIfEmpty(p.FirstName), nullIfEmpty(p.MiddleName), nullIfEmpty(p.LastName),
		nullIfEmpty(p.OrgAccountBalance), nullIfEmpty(p.ThirdPartyTransID), raw, paymentID,
	)
	if err != nil {
		return err
	}
	if s.riskHook != nil {
		et := "payment"
		eid := paymentID
		title := fmt.Sprintf("C2B amount mismatch: expected KES %.0f got %.0f", expected, p.Amount)
		_ = s.riskHook.CreateRiskAlert(ctx, tenantID, nil, "c2b_amount_mismatch", "high", title, &et, &eid, map[string]any{
			"trans_id":        p.TransID,
			"expected_amount": expected,
			"received_amount": p.Amount,
			"bill_ref_number": p.BillRefNumber,
		})
	}
	return nil
}

func (s *Service) ListC2BTransactions(ctx context.Context, tenantID uuid.UUID, status string) ([]C2BTransaction, error) {
	q := `
		SELECT id, payment_id, COALESCE(trans_id, ''), COALESCE(amount, 0)::float8,
		       COALESCE(business_shortcode, ''), COALESCE(bill_ref_number, ''),
		       COALESCE(msisdn, ''), status, created_at
		FROM payments.mpesa_c2b_transactions
		WHERE tenant_id = $1`
	args := []any{tenantID}
	if status != "" {
		q += ` AND status = $2`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []C2BTransaction
	for rows.Next() {
		var t C2BTransaction
		if err := rows.Scan(&t.ID, &t.PaymentID, &t.TransID, &t.Amount, &t.BusinessShort, &t.BillRefNumber, &t.MSISDN, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []C2BTransaction{}
	}
	return items, nil
}

// MatchC2BToPayment links an unmatched/mismatch C2B row to a payment and confirms it.
func (s *Service) MatchC2BToPayment(ctx context.Context, tenantID, c2bID, paymentID, actorID uuid.UUID) (*Payment, error) {
	var status string
	var transID string
	err := s.pool.QueryRow(ctx, `
		SELECT status, COALESCE(trans_id, '')
		FROM payments.mpesa_c2b_transactions
		WHERE tenant_id = $1 AND id = $2`, tenantID, c2bID).Scan(&status, &transID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("c2b transaction not found")
		}
		return nil, err
	}
	if status != "unmatched" && status != "amount_mismatch" {
		return nil, fmt.Errorf("c2b transaction is not unmatched")
	}

	var method, payStatus string
	err = s.pool.QueryRow(ctx, `
		SELECT method, status FROM payments.payments
		WHERE tenant_id = $1 AND id = $2`, tenantID, paymentID).Scan(&method, &payStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, err
	}
	if method != "mpesa_c2b" {
		return nil, fmt.Errorf("match target must be an mpesa_c2b payment")
	}
	if payStatus == "allocated" || payStatus == "confirmed" {
		return nil, fmt.Errorf("payment already settled")
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE payments.mpesa_c2b_transactions
		SET payment_id = $1, status = 'pending', updated_at = now()
		WHERE id = $2`, paymentID, c2bID)
	if err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE payments.mpesa_c2b_transactions SET status = 'superseded', updated_at = now()
		WHERE payment_id = $1 AND id <> $2 AND status IN ('pending', 'awaiting', 'initiated', 'amount_mismatch')`,
		paymentID, c2bID)
	ref := transID
	if ref == "" {
		ref = c2bID.String()
	}
	pay, err := s.ConfirmMpesaWebhook(ctx, tenantID, paymentID, ref)
	if err != nil {
		return nil, err
	}
	if s.riskHook != nil {
		_, _ = s.riskHook.ResolveOpenAlertsByEntity(ctx, tenantID, "unmatched_c2b", c2bID, actorID)
		_, _ = s.riskHook.ResolveOpenAlertsByEntity(ctx, tenantID, "c2b_amount_mismatch", paymentID, actorID)
	}
	return pay, nil
}
