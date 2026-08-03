package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/packages/pkg/events"
)

type Service struct {
	pool             *pgxpool.Pool
	bus              *events.Bus
	riskHook         RiskHook
	orderPaidHook    OrderPaidHook
	repairPaidHook   RepairPaidHook
	salePaidHook     SalePaidHook
	outstanding      OutstandingResolver
	quickSaleCreator QuickSaleCreator
}

// RiskHook raises and clears leakage alerts.
type RiskHook interface {
	CreateRiskAlert(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, kind, severity, title string, entityType *string, entityID *uuid.UUID, details map[string]any) error
	ResolveOpenAlertsByEntity(ctx context.Context, tenantID uuid.UUID, kind string, entityID, resolver uuid.UUID) (int64, error)
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) SetOrderPaidHook(h OrderPaidHook) {
	s.orderPaidHook = h
}

func (s *Service) SetRepairPaidHook(h RepairPaidHook) {
	s.repairPaidHook = h
}

func (s *Service) SetSalePaidHook(h SalePaidHook) {
	s.salePaidHook = h
}

// SalePaidHook completes a draft POS sale after its payment allocates (STK webhook path).
type SalePaidHook interface {
	OnSalePaymentSettled(ctx context.Context, tenantID, saleID, actorID uuid.UUID) error
}

func (s *Service) SetOutstandingResolver(fn OutstandingResolver) {
	s.outstanding = fn
}

func (s *Service) SetQuickSaleCreator(c QuickSaleCreator) {
	s.quickSaleCreator = c
}

// RepairPaidHook advances a completed repair to collected when balance is settled.
type RepairPaidHook interface {
	OnRepairPaymentSettled(ctx context.Context, tenantID, repairID, actorID uuid.UUID) error
}

// RepairSettledAdapter wires repair.TryMarkCollectedIfSettled after payments land.
type RepairSettledAdapter struct {
	Svc interface {
		TryMarkCollectedIfSettled(ctx context.Context, tenantID, repairID, actorID, corrID uuid.UUID)
	}
}

func (a RepairSettledAdapter) OnRepairPaymentSettled(ctx context.Context, tenantID, repairID, actorID uuid.UUID) error {
	a.Svc.TryMarkCollectedIfSettled(ctx, tenantID, repairID, actorID, uuid.New())
	return nil
}

func (s *Service) SetRiskHook(h RiskHook) {
	s.riskHook = h
}

func (s *Service) SetEventBus(bus *events.Bus) {
	s.bus = bus
}

func (s *Service) publishPaymentConfirmed(ctx context.Context, tenantID, paymentID, actorID, corrID uuid.UUID) {
	if s.bus == nil {
		return
	}
	env := events.New("payment.confirmed", tenantID, corrID, map[string]any{
		"payment_id": paymentID.String(),
	})
	var method string
	if qErr := s.pool.QueryRow(ctx, `
		SELECT method FROM payments.payments WHERE tenant_id = $1 AND id = $2`, tenantID, paymentID).Scan(&method); qErr == nil && method != "" {
		env.Payload["method"] = method
	}
	if actorID != uuid.Nil {
		env.ActorID = &actorID
	}
	s.bus.Publish(env)
}

func (s *Service) notifyPayableHooks(ctx context.Context, tenantID, paymentID, actorID uuid.UUID) {
	var payableType, payStatus string
	var payableID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT a.payable_type, a.payable_id, p.status
		FROM payments.payment_allocations a
		JOIN payments.payments p ON p.id = a.payment_id
		WHERE a.payment_id = $1 LIMIT 1`, paymentID).Scan(&payableType, &payableID, &payStatus)
	if err != nil {
		return
	}
	// Online orders only unlock after the payment is fully allocated.
	if payableType == "order" && s.orderPaidHook != nil &&
		(payStatus == "allocated" || payStatus == "confirmed") {
		_ = s.orderPaidHook.OnOrderPaid(ctx, tenantID, payableID, actorID)
	}
	// POS sales: complete draft + deduct stock once STK/C2B money is allocated.
	if payableType == "sale" && s.salePaidHook != nil &&
		(payStatus == "allocated" || payStatus == "confirmed") {
		_ = s.salePaidHook.OnSalePaymentSettled(ctx, tenantID, payableID, actorID)
	}
	// Repairs: settled cash/credit counts toward handoff; digital waits until allocated.
	if payableType == "repair" && s.repairPaidHook != nil {
		_ = s.repairPaidHook.OnRepairPaymentSettled(ctx, tenantID, payableID, actorID)
	}
}

type Payment struct {
	ID                uuid.UUID  `json:"id"`
	Method            string     `json:"method"`
	Amount            float64    `json:"amount"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at,omitempty"`
	CheckoutRequestID string     `json:"checkout_request_id,omitempty"`
	Phone             string     `json:"phone,omitempty"`
	AccountRef        string     `json:"account_reference,omitempty"`
	PayableType       string     `json:"payable_type,omitempty"`
	PayableID         *uuid.UUID `json:"payable_id,omitempty"`
	JobCode           string     `json:"job_code,omitempty"`
	CustomerID        *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName      string     `json:"customer_name,omitempty"`
	SaleLabel         string     `json:"sale_label,omitempty"`
}

type Refund struct {
	ID        uuid.UUID  `json:"id"`
	PaymentID uuid.UUID  `json:"payment_id"`
	Amount    float64    `json:"amount"`
	Status    string     `json:"status"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedBy uuid.UUID  `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at,omitempty"`
}

var ErrSelfApproveRefund = errors.New("cannot approve your own refund request")

type CreatePaymentInput struct {
	TenantID    uuid.UUID
	BranchID    *uuid.UUID
	Method      string
	Amount      float64
	Currency    string
	PayableType string
	PayableID   uuid.UUID
	Phone       string
	AccountRef  string
	CustomerID  *uuid.UUID
	ClientID    *uuid.UUID // optional client-generated payment id (offline sync)
	ActorID     uuid.UUID
	CorrID      uuid.UUID
	BodyHash    string // optional sync payload hash for cash idempotency_records
	BalanceDue  float64 // when >0, reject overpayment
}

func (s *Service) paymentByCorrelation(ctx context.Context, tenantID, corrID uuid.UUID) (*Payment, error) {
	var p Payment
	err := s.pool.QueryRow(ctx, `
		SELECT id, method, amount::float8, status FROM payments.payments
		WHERE tenant_id = $1 AND correlation_id = $2`, tenantID, corrID).
		Scan(&p.ID, &p.Method, &p.Amount, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) recordCashIdempotency(ctx context.Context, tenantID, actionID uuid.UUID, bodyHash string, p *Payment) {
	if actionID == uuid.Nil || p == nil {
		return
	}
	resp, _ := json.Marshal(p)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO payments.idempotency_records (action_id, tenant_id, body_hash, status_code, response)
		VALUES ($1, $2, $3, 200, $4)
		ON CONFLICT (action_id) DO NOTHING`,
		actionID, tenantID, bodyHash, resp)
}

func (s *Service) CreatePayment(ctx context.Context, in CreatePaymentInput) (*Payment, error) {
	if in.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if in.Currency == "" {
		in.Currency = "KES"
	}
	if in.Method == "card" {
		return nil, fmt.Errorf("card payments are not configured")
	}
	if IsDigitalMethod(in.Method) {
		if err := s.RequireDigitalReady(ctx, in.TenantID, in.Method); err != nil {
			return nil, err
		}
	}
	if in.Method == "mpesa_stk" && strings.TrimSpace(in.Phone) == "" {
		return nil, fmt.Errorf("phone required for M-Pesa STK")
	}
	if in.Method == "mpesa_c2b" && strings.TrimSpace(in.AccountRef) == "" {
		return nil, fmt.Errorf("account_reference (bill ref) required for C2B")
	}
	if in.Method == "store_credit" {
		if in.CustomerID == nil || *in.CustomerID == uuid.Nil {
			return nil, fmt.Errorf("customer_id required for store credit")
		}
	}
	if err := s.assertAmountWithinBalance(ctx, in.TenantID, in.PayableType, in.PayableID, in.Amount, in.BalanceDue); err != nil {
		return nil, err
	}

	if in.CorrID != uuid.Nil {
		if replay, err := s.paymentByCorrelation(ctx, in.TenantID, in.CorrID); err != nil {
			return nil, err
		} else if replay != nil {
			return replay, nil
		}
		if IsCashMethod(in.Method) {
			var cached []byte
			err := s.pool.QueryRow(ctx, `
				SELECT response FROM payments.idempotency_records
				WHERE action_id = $1 AND tenant_id = $2`, in.CorrID, in.TenantID).Scan(&cached)
			if err == nil && len(cached) > 0 {
				var p Payment
				if json.Unmarshal(cached, &p) == nil && p.ID != uuid.Nil {
					return &p, nil
				}
			} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
	}

	status := InitialPaymentStatus(in.Method)
	id := uuid.New()
	if in.ClientID != nil && *in.ClientID != uuid.Nil {
		id = *in.ClientID
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO payments.payments (id, tenant_id, branch_id, method, amount, currency, status, received_by, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)
		ON CONFLICT (id) DO NOTHING`,
		id, in.TenantID, in.BranchID, in.Method, in.Amount, in.Currency, status, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var replay Payment
		err = tx.QueryRow(ctx, `
			SELECT id, method, amount::float8, status FROM payments.payments
			WHERE tenant_id = $1 AND id = $2`, in.TenantID, id).
			Scan(&replay.ID, &replay.Method, &replay.Amount, &replay.Status)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &replay, nil
	}
	allocID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO payments.payment_allocations (id, tenant_id, payment_id, payable_type, payable_id, amount)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		allocID, in.TenantID, id, in.PayableType, in.PayableID, in.Amount)
	if err != nil {
		return nil, err
	}
	if in.Method == "mpesa_stk" {
		_, err = tx.Exec(ctx, `
			INSERT INTO payments.mpesa_stk_transactions (id, tenant_id, payment_id, status, phone, account_reference)
			VALUES ($1, $2, $3, 'pending', $4, $5)`,
			uuid.New(), in.TenantID, id, nullIfEmpty(in.Phone), nullIfEmpty(in.AccountRef))
		if err != nil {
			return nil, err
		}
	} else if in.Method == "mpesa_c2b" {
		_, err = tx.Exec(ctx, `
			INSERT INTO payments.mpesa_c2b_transactions (id, tenant_id, payment_id, status, msisdn, bill_ref_number)
			VALUES ($1, $2, $3, 'pending', $4, $5)`,
			uuid.New(), in.TenantID, id, nullIfEmpty(in.Phone), nullIfEmpty(in.AccountRef))
		if err != nil {
			return nil, err
		}
	} else if in.Method == "bank_paybill" || in.Method == "bank_transfer" {
		paybill, account := "", in.AccountRef
		if cfg, cfgErr := s.GetProviderSettings(ctx, in.TenantID); cfgErr == nil {
			paybill = cfg.BankPaybill
			if account == "" {
				account = cfg.BankAccount
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO payments.bank_transactions (id, tenant_id, payment_id, paybill, account_number, status)
			VALUES ($1, $2, $3, $4, $5, 'pending')`,
			uuid.New(), in.TenantID, id, nullIfEmpty(paybill), nullIfEmpty(account))
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if in.Method == "store_credit" {
		if err := s.applyStoreCredit(ctx, in.TenantID, *in.CustomerID, id, in.ActorID, in.Amount); err != nil {
			_, _ = s.pool.Exec(ctx, `UPDATE payments.payments SET status = 'failed', updated_at = now() WHERE id = $1`, id)
			return nil, err
		}
		_, _ = s.pool.Exec(ctx, `
			UPDATE payments.payments SET status = 'allocated', updated_at = now(), version = version + 1 WHERE id = $1`, id)
		status = "allocated"
	}

	out := &Payment{ID: id, Method: in.Method, Amount: in.Amount, Status: status, Phone: in.Phone, AccountRef: in.AccountRef}
	if in.Method == "mpesa_stk" {
		acct := in.AccountRef
		if acct == "" {
			acct = in.PayableType
		}
		checkout, merchant, stkErr := s.InitiateSTKPush(ctx, in.TenantID, id, in.Amount, in.Phone, acct)
		if stkErr != nil {
			_, _ = s.pool.Exec(ctx, `
				UPDATE payments.mpesa_stk_transactions SET status = 'failed', result_code = $1, updated_at = now()
				WHERE payment_id = $2`, truncate(stkErr.Error(), 120), id)
			_, _ = s.pool.Exec(ctx, `
				UPDATE payments.payments SET status = 'failed', updated_at = now() WHERE id = $1`, id)
			return nil, stkErr
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE payments.mpesa_stk_transactions
			SET checkout_request_id = $1, merchant_request_id = $2, status = 'stk_sent', updated_at = now()
			WHERE payment_id = $3`, checkout, merchant, id)
		if err != nil {
			return nil, err
		}
		out.CheckoutRequestID = checkout
		out.Status = "initiated"
	}
	s.notifyPayableHooks(ctx, in.TenantID, id, in.ActorID)
	if IsCashMethod(in.Method) && in.CorrID != uuid.Nil {
		s.recordCashIdempotency(ctx, in.TenantID, in.CorrID, in.BodyHash, out)
	}
	return out, nil
}

func nullIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) ConfirmMpesaWebhook(ctx context.Context, tenantID, paymentID uuid.UUID, providerRef string) (*Payment, error) {
	var method, status string
	var amount float64
	var currentRef *string
	err := s.pool.QueryRow(ctx, `
		SELECT method, amount, status, provider_ref FROM payments.payments WHERE tenant_id = $1 AND id = $2`, tenantID, paymentID).
		Scan(&method, &amount, &status, &currentRef)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, err
	}
	if !IsDigitalMethod(method) {
		return nil, fmt.Errorf("not a digital payment")
	}

	// Upgrade stored refs when the STK callback later supplies the real M-Pesa receipt.
	betterRef := CustomerFacingPaymentRef(providerRef)
	if betterRef != "" && (status == "allocated" || status == "confirmed") {
		cur := ""
		if currentRef != nil {
			cur = *currentRef
		}
		if IsDarajaCheckoutRef(cur) || strings.TrimSpace(cur) == "" {
			_, _ = s.pool.Exec(ctx, `
				UPDATE payments.payments SET provider_ref = $1, updated_at = now() WHERE id = $2`, betterRef, paymentID)
			if method == "mpesa_stk" {
				_, _ = s.pool.Exec(ctx, `
					UPDATE payments.mpesa_stk_transactions
					SET mpesa_receipt = $1, status = 'confirmed', updated_at = now()
					WHERE payment_id = $2`, betterRef, paymentID)
			}
		}
		s.afterDigitalPaymentSettled(ctx, tenantID, paymentID, method, uuid.Nil)
		return &Payment{ID: paymentID, Method: method, Amount: amount, Status: status}, nil
	}

	if status == "allocated" || status == "confirmed" {
		s.afterDigitalPaymentSettled(ctx, tenantID, paymentID, method, uuid.Nil)
		return &Payment{ID: paymentID, Method: method, Amount: amount, Status: status}, nil
	}

	storeRef := betterRef
	if storeRef == "" {
		storeRef = strings.TrimSpace(providerRef)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE payments.payments SET status = 'confirmed', provider_ref = $1, updated_at = now(), version = version + 1
		WHERE id = $2`, storeRef, paymentID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE payments.payments SET status = 'allocated', updated_at = now(), version = version + 1
		WHERE id = $1`, paymentID)
	if err != nil {
		return nil, err
	}
	switch method {
	case "mpesa_stk":
		_, err = tx.Exec(ctx, `
			UPDATE payments.mpesa_stk_transactions
			SET status = 'confirmed', mpesa_receipt = $1, updated_at = now()
			WHERE payment_id = $2`, storeRef, paymentID)
	case "mpesa_c2b":
		_, err = tx.Exec(ctx, `
			UPDATE payments.mpesa_c2b_transactions
			SET status = 'confirmed', trans_id = COALESCE(NULLIF(trans_id, ''), $1), updated_at = now()
			WHERE payment_id = $2 AND status IS DISTINCT FROM 'superseded'`, storeRef, paymentID)
	case "bank_paybill", "bank_transfer":
		_, err = tx.Exec(ctx, `
			UPDATE payments.bank_transactions
			SET status = 'confirmed', provider_ref = $1, updated_at = now()
			WHERE payment_id = $2`, storeRef, paymentID)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.riskHook != nil {
		_, _ = s.riskHook.ResolveOpenAlertsByEntity(ctx, tenantID, "unverified_payment", paymentID, uuid.Nil)
	}
	// Customer attach before sale completion so receipts can show a resolved name.
	s.afterDigitalPaymentSettled(ctx, tenantID, paymentID, method, uuid.Nil)
	s.publishPaymentConfirmed(ctx, tenantID, paymentID, uuid.Nil, uuid.New())
	return &Payment{ID: paymentID, Method: method, Amount: amount, Status: "allocated"}, nil
}

// afterDigitalPaymentSettled attaches a customer (when possible) then runs payable hooks
// such as completing a draft POS sale. Order matters: customer_id must land before CompleteSale.
func (s *Service) afterDigitalPaymentSettled(ctx context.Context, tenantID, paymentID uuid.UUID, method string, actorID uuid.UUID) {
	_ = s.attachCustomerForDigitalPayment(ctx, tenantID, paymentID, method)
	s.notifyPayableHooks(ctx, tenantID, paymentID, actorID)
}

func (s *Service) ConfirmMpesaByCheckout(ctx context.Context, tenantID uuid.UUID, checkoutRequestID, providerRef string) (*Payment, error) {
	var paymentID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT payment_id FROM payments.mpesa_stk_transactions
		WHERE tenant_id = $1 AND checkout_request_id = $2`, tenantID, checkoutRequestID).Scan(&paymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("stk transaction not found")
		}
		return nil, err
	}
	if providerRef == "" {
		providerRef = checkoutRequestID
	}
	return s.ConfirmMpesaWebhook(ctx, tenantID, paymentID, providerRef)
}

func (s *Service) CreateRefund(ctx context.Context, tenantID, paymentID, actorID, corrID uuid.UUID, amount float64, reason *string, approvedBy *uuid.UUID) (*Refund, error) {
	var payAmount float64
	var payStatus string
	err := s.pool.QueryRow(ctx, `SELECT amount::float8, status FROM payments.payments WHERE tenant_id = $1 AND id = $2`, tenantID, paymentID).Scan(&payAmount, &payStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, err
	}
	if payStatus != "allocated" && payStatus != "confirmed" {
		return nil, fmt.Errorf("only allocated/confirmed payments can be refunded")
	}
	if amount <= 0 || amount > payAmount {
		return nil, fmt.Errorf("invalid refund amount")
	}
	status := "pending"
	if approvedBy != nil {
		status = "approved"
	}
	id := uuid.New()
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO payments.refunds (id, tenant_id, payment_id, amount, status, reason, approved_by, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, tenantID, paymentID, amount, status, reason, approvedBy, actorID, corrID)
	if err != nil {
		return nil, err
	}
	return &Refund{ID: id, PaymentID: paymentID, Amount: amount, Status: status, Reason: reason, CreatedBy: actorID, CreatedAt: now}, nil
}

func (s *Service) ListRefunds(ctx context.Context, tenantID uuid.UUID, status string) ([]Refund, error) {
	q := `
		SELECT id, payment_id, amount::float8, status, reason, COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid), created_at
		FROM payments.refunds WHERE tenant_id = $1`
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
	var items []Refund
	for rows.Next() {
		var r Refund
		if err := rows.Scan(&r.ID, &r.PaymentID, &r.Amount, &r.Status, &r.Reason, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if items == nil {
		items = []Refund{}
	}
	return items, nil
}

func (s *Service) ApproveRefund(ctx context.Context, tenantID, refundID, approver uuid.UUID) (*Refund, error) {
	var r Refund
	var reason *string
	var createdBy uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id, payment_id, amount::float8, status, reason, COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid), created_at
		FROM payments.refunds WHERE tenant_id = $1 AND id = $2`, tenantID, refundID).
		Scan(&r.ID, &r.PaymentID, &r.Amount, &r.Status, &reason, &createdBy, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("refund not found")
		}
		return nil, err
	}
	if r.Status != "pending" {
		return nil, fmt.Errorf("refund not pending")
	}
	if err := ValidateRefundApprove(createdBy.String(), approver.String()); err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `UPDATE payments.refunds SET status = 'approved', approved_by = $1 WHERE id = $2`, approver, refundID)
	if err != nil {
		return nil, err
	}
	r.Status = "approved"
	r.Reason = reason
	r.CreatedBy = createdBy
	return &r, nil
}

func ValidateRefundApprove(creatorID, approverID string) error {
	if creatorID != "" && creatorID != "00000000-0000-0000-0000-000000000000" && creatorID == approverID {
		return ErrSelfApproveRefund
	}
	return nil
}

func (s *Service) ListPayments(ctx context.Context, tenantID uuid.UUID, limit int) ([]Payment, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.method, p.amount::float8, p.status, p.created_at,
		       COALESCE(stk.account_reference, c2b.bill_ref_number, bank.account_number, ''),
		       COALESCE(stk.phone, c2b.msisdn, ''),
		       COALESCE(a.payable_type, ''),
		       a.payable_id,
		       COALESCE(j.job_code, ''),
		       COALESCE(j.customer_id, sc.customer_id),
		       COALESCE(cust.full_name, scust.full_name, ''),
		       CASE WHEN a.payable_type = 'sale' THEN COALESCE('Sale ' || LEFT(a.payable_id::text, 8), '') ELSE '' END
		FROM payments.payments p
		LEFT JOIN LATERAL (
			SELECT payable_type, payable_id
			FROM payments.payment_allocations
			WHERE payment_id = p.id
			ORDER BY created_at DESC
			LIMIT 1
		) a ON true
		LEFT JOIN LATERAL (
			SELECT account_reference, phone FROM payments.mpesa_stk_transactions
			WHERE payment_id = p.id
			ORDER BY created_at DESC LIMIT 1
		) stk ON true
		LEFT JOIN LATERAL (
			SELECT bill_ref_number, msisdn FROM payments.mpesa_c2b_transactions
			WHERE payment_id = p.id AND status IS DISTINCT FROM 'superseded'
			ORDER BY created_at DESC LIMIT 1
		) c2b ON true
		LEFT JOIN LATERAL (
			SELECT account_number FROM payments.bank_transactions
			WHERE payment_id = p.id
			ORDER BY created_at DESC LIMIT 1
		) bank ON true
		LEFT JOIN repair.repair_jobs j ON a.payable_type = 'repair' AND j.id = a.payable_id AND j.tenant_id = p.tenant_id
		LEFT JOIN repair.customers cust ON cust.id = j.customer_id
		LEFT JOIN payments.store_credits sc ON a.payable_type = 'store_credit' AND sc.id = a.payable_id
		LEFT JOIN repair.customers scust ON scust.id = sc.customer_id
		WHERE p.tenant_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Payment
	for rows.Next() {
		var p Payment
		var payableID, customerID *uuid.UUID
		if err := rows.Scan(
			&p.ID, &p.Method, &p.Amount, &p.Status, &p.CreatedAt,
			&p.AccountRef, &p.Phone, &p.PayableType, &payableID,
			&p.JobCode, &customerID, &p.CustomerName, &p.SaleLabel,
		); err != nil {
			return nil, err
		}
		p.PayableID = payableID
		p.CustomerID = customerID
		items = append(items, p)
	}
	return items, nil
}

func (s *Service) GetPayment(ctx context.Context, tenantID, paymentID uuid.UUID) (*Payment, error) {
	var p Payment
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, p.method, p.amount::float8, p.status,
		       COALESCE(stk.account_reference, c2b.bill_ref_number, bank.account_number, '')
		FROM payments.payments p
		LEFT JOIN payments.mpesa_stk_transactions stk ON stk.payment_id = p.id
		LEFT JOIN payments.mpesa_c2b_transactions c2b ON c2b.payment_id = p.id AND c2b.status IS DISTINCT FROM 'superseded'
		LEFT JOIN payments.bank_transactions bank ON bank.payment_id = p.id
		WHERE p.tenant_id = $1 AND p.id = $2
		LIMIT 1`, tenantID, paymentID).
		Scan(&p.ID, &p.Method, &p.Amount, &p.Status, &p.AccountRef)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, err
	}
	return &p, nil
}

func (s *Service) ListPaymentsForPayable(ctx context.Context, tenantID uuid.UUID, payableType string, payableID uuid.UUID) ([]Payment, error) {
	// mpesa_c2b_transactions has no UNIQUE(payment_id) constraint (a payment can pick up
	// a superseded/duplicate webhook row), so a plain LEFT JOIN can fan out and return the
	// same payment twice. Use LATERAL subqueries capped at one row each to guarantee a
	// single result row per payment regardless of how many child rows exist.
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.method, p.amount::float8, p.status,
		       COALESCE(stk.checkout_request_id, ''), COALESCE(stk.phone, c2b.msisdn, '')
		FROM payments.payments p
		LEFT JOIN LATERAL (
			SELECT checkout_request_id, phone FROM payments.mpesa_stk_transactions
			WHERE payment_id = p.id
			ORDER BY created_at DESC LIMIT 1
		) stk ON true
		LEFT JOIN LATERAL (
			SELECT msisdn FROM payments.mpesa_c2b_transactions
			WHERE payment_id = p.id AND status IS DISTINCT FROM 'superseded'
			ORDER BY created_at DESC LIMIT 1
		) c2b ON true
		WHERE p.tenant_id = $1
		  AND EXISTS (
		    SELECT 1 FROM payments.payment_allocations a
		    WHERE a.payment_id = p.id AND a.payable_type = $2 AND a.payable_id = $3
		  )
		ORDER BY p.created_at DESC`, tenantID, payableType, payableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.Method, &p.Amount, &p.Status, &p.CheckoutRequestID, &p.Phone); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, nil
}
