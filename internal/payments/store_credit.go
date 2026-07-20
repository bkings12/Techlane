package payments

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type StoreCredit struct {
	CustomerID uuid.UUID `json:"customer_id"`
	Balance    float64   `json:"balance"`
	Currency   string    `json:"currency"`
}

func (s *Service) GetStoreCredit(ctx context.Context, tenantID, customerID uuid.UUID) (*StoreCredit, error) {
	var bal float64
	var currency string
	err := s.pool.QueryRow(ctx, `
		SELECT balance::float8, currency FROM payments.store_credits
		WHERE tenant_id = $1 AND customer_id = $2`, tenantID, customerID).Scan(&bal, &currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &StoreCredit{CustomerID: customerID, Balance: 0, Currency: "KES"}, nil
		}
		return nil, err
	}
	return &StoreCredit{CustomerID: customerID, Balance: bal, Currency: currency}, nil
}

func (s *Service) IssueStoreCredit(ctx context.Context, tenantID, customerID, actorID uuid.UUID, amount float64, note string) (*StoreCredit, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO payments.store_credits (id, tenant_id, customer_id, balance, currency, updated_at)
		VALUES ($1, $2, $3, $4, 'KES', now())
		ON CONFLICT (tenant_id, customer_id) DO UPDATE
		SET balance = payments.store_credits.balance + EXCLUDED.balance, updated_at = now()`,
		uuid.New(), tenantID, customerID, amount)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO payments.store_credit_ledger (id, tenant_id, customer_id, amount, entry_type, note, created_by)
		VALUES ($1, $2, $3, $4, 'issue', $5, $6)`,
		uuid.New(), tenantID, customerID, amount, nullIfEmpty(note), actorID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetStoreCredit(ctx, tenantID, customerID)
}

func (s *Service) applyStoreCredit(ctx context.Context, tenantID, customerID, paymentID, actorID uuid.UUID, amount float64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var bal float64
	err = tx.QueryRow(ctx, `
		SELECT balance::float8 FROM payments.store_credits
		WHERE tenant_id = $1 AND customer_id = $2 FOR UPDATE`, tenantID, customerID).Scan(&bal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no store credit for customer")
		}
		return err
	}
	if amount > bal+0.009 {
		return fmt.Errorf("store credit balance %.2f insufficient for %.2f", bal, amount)
	}
	_, err = tx.Exec(ctx, `
		UPDATE payments.store_credits SET balance = balance - $1, updated_at = now()
		WHERE tenant_id = $2 AND customer_id = $3`, amount, tenantID, customerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO payments.store_credit_ledger (id, tenant_id, customer_id, amount, entry_type, payment_id, created_by)
		VALUES ($1, $2, $3, $4, 'redeem', $5, $6)`,
		uuid.New(), tenantID, customerID, -amount, paymentID, actorID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ConfirmBankPayment(ctx context.Context, tenantID, paymentID uuid.UUID, providerRef string, actorID uuid.UUID) (*Payment, error) {
	var method string
	err := s.pool.QueryRow(ctx, `SELECT method FROM payments.payments WHERE tenant_id = $1 AND id = $2`,
		tenantID, paymentID).Scan(&method)
	if err != nil {
		return nil, fmt.Errorf("payment not found")
	}
	if method != "bank_paybill" && method != "bank_transfer" {
		return nil, fmt.Errorf("not a bank payment")
	}
	p, err := s.ConfirmMpesaWebhook(ctx, tenantID, paymentID, providerRef)
	if err != nil {
		return nil, err
	}
	_ = actorID
	return p, nil
}
