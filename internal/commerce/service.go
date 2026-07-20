package commerce

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/inventory"
)

// PaymentTaker records digital payment against an online order.
type PaymentTaker interface {
	TakeOrderPayment(ctx context.Context, in OrderPaymentInput) (*OrderPaymentResult, error)
}

type OrderPaymentInput struct {
	TenantID   uuid.UUID
	BranchID   uuid.UUID
	OrderID    uuid.UUID
	Method     string
	Amount     float64
	Phone      string
	AccountRef string
	ActorID    uuid.UUID
	CorrID     uuid.UUID
}

type OrderPaymentResult struct {
	ID         uuid.UUID `json:"id"`
	Method     string    `json:"method"`
	Amount     float64   `json:"amount"`
	Status     string    `json:"status"`
	AccountRef string    `json:"account_reference,omitempty"`
}

// Service scaffolds e-commerce on shared catalog/inventory/payments (BOCP-first).
type Service struct {
	pool     *pgxpool.Pool
	inv      *inventory.Service
	payments PaymentTaker
}

func NewService(pool *pgxpool.Pool, inv *inventory.Service) *Service {
	return &Service{pool: pool, inv: inv}
}

func (s *Service) SetPaymentTaker(p PaymentTaker) {
	s.payments = p
}

type CartItemInput struct {
	VariantID uuid.UUID `json:"variant_id"`
	Quantity  int       `json:"quantity"`
}

type CheckoutRequest struct {
	CustomerID      *uuid.UUID      `json:"customer_id"`
	BranchID        uuid.UUID       `json:"branch_id"`
	LocationID      uuid.UUID       `json:"location_id"`
	Items           []CartItemInput `json:"items"`
	FulfilmentType  string          `json:"fulfilment_type"`
	ReservationTTLM int             `json:"reservation_ttl_minutes"`
	Method          string          `json:"method"`
	Phone           string          `json:"phone"`
	ActorID         uuid.UUID       `json:"-"`
	CorrID          uuid.UUID       `json:"-"`
}

type Order struct {
	ID             uuid.UUID           `json:"id"`
	Status         string              `json:"status"`
	CollectionCode string              `json:"collection_code,omitempty"`
	Total          float64             `json:"total"`
	BranchID       *uuid.UUID          `json:"branch_id,omitempty"`
	FulfilmentType string              `json:"fulfilment_type,omitempty"`
	CreatedAt      time.Time           `json:"created_at,omitempty"`
	Payment        *OrderPaymentResult `json:"payment,omitempty"`
}

type CheckoutResult struct {
	Order   *Order              `json:"order"`
	Payment *OrderPaymentResult `json:"payment,omitempty"`
}

// StartCheckout reserves stock for all lines and creates a pending_payment order.
func (s *Service) StartCheckout(ctx context.Context, tenantID uuid.UUID, req CheckoutRequest) (*CheckoutResult, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("empty cart")
	}
	if req.BranchID == uuid.Nil || req.LocationID == uuid.Nil {
		return nil, fmt.Errorf("branch_id and location_id required")
	}
	if req.FulfilmentType == "" {
		req.FulfilmentType = "branch_pickup"
	}
	if req.Method == "" {
		req.Method = "mpesa_c2b"
	}
	ttl := req.ReservationTTLM
	if ttl <= 0 {
		ttl = 15
	}

	var subtotal float64
	orderID := uuid.New()
	var firstResID uuid.UUID
	reserved := make([]uuid.UUID, 0, len(req.Items))

	rollback := func() {
		_ = s.inv.ReleaseReservationsByReference(ctx, tenantID, "order", orderID)
	}

	for i, it := range req.Items {
		if it.Quantity <= 0 {
			rollback()
			return nil, fmt.Errorf("quantity must be positive")
		}
		p, err := s.inv.GetVariantPrice(ctx, tenantID, it.VariantID)
		if err != nil {
			rollback()
			return nil, err
		}
		subtotal += p * float64(it.Quantity)
		res, err := s.inv.ReserveInventory(ctx, tenantID, it.VariantID, req.LocationID, it.Quantity, time.Duration(ttl)*time.Minute, "order", orderID)
		if err != nil {
			rollback()
			return nil, err
		}
		reserved = append(reserved, res.ID)
		if i == 0 {
			firstResID = res.ID
		}
	}
	_ = reserved

	code, err := randomCode(6)
	if err != nil {
		rollback()
		return nil, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO sales.orders (
			id, tenant_id, branch_id, customer_id, channel, status, collection_code,
			fulfilment_type, subtotal, total, reservation_id
		) VALUES ($1,$2,$3,$4,'online','pending_payment',$5,$6,$7,$7,$8)
	`, orderID, tenantID, req.BranchID, req.CustomerID, code, req.FulfilmentType, subtotal, firstResID)
	if err != nil {
		rollback()
		return nil, err
	}

	for _, it := range req.Items {
		p, _ := s.inv.GetVariantPrice(ctx, tenantID, it.VariantID)
		_, err = s.pool.Exec(ctx, `
			INSERT INTO sales.order_items (id, tenant_id, order_id, variant_id, quantity, unit_price, line_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, uuid.New(), tenantID, orderID, it.VariantID, it.Quantity, p, p*float64(it.Quantity))
		if err != nil {
			rollback()
			return nil, err
		}
	}

	order := &Order{
		ID:             orderID,
		Status:         "pending_payment",
		Total:          subtotal,
		BranchID:       &req.BranchID,
		FulfilmentType: req.FulfilmentType,
		CreatedAt:      time.Now().UTC(),
	}

	result := &CheckoutResult{Order: order}
	if s.payments != nil && (req.Method == "mpesa_stk" || req.Method == "mpesa_c2b" || req.Method == "bank_paybill") {
		acct := "ORD-" + strings.ToUpper(orderID.String()[:8])
		pay, payErr := s.payments.TakeOrderPayment(ctx, OrderPaymentInput{
			TenantID: tenantID, BranchID: req.BranchID, OrderID: orderID,
			Method: req.Method, Amount: subtotal, Phone: req.Phone, AccountRef: acct,
			ActorID: req.ActorID, CorrID: req.CorrID,
		})
		if payErr != nil {
			rollback()
			_, _ = s.pool.Exec(ctx, `UPDATE sales.orders SET status='cancelled', updated_at=now() WHERE id=$1`, orderID)
			return nil, fmt.Errorf("payment failed: %w", payErr)
		}
		result.Payment = pay
		order.Payment = pay
		if pay.Status == "allocated" {
			if err := s.ConfirmPaid(ctx, tenantID, orderID, req.ActorID); err != nil {
				return nil, err
			}
			fresh, _ := s.GetOrder(ctx, tenantID, orderID)
			if fresh != nil {
				result.Order = fresh
				result.Order.Payment = pay
			}
		}
	}

	return result, nil
}

// ConfirmPaid converts all order reservations and marks ready_for_pickup for BOCP.
func (s *Service) ConfirmPaid(ctx context.Context, tenantID, orderID, actorID uuid.UUID) error {
	var status, fulfilment string
	err := s.pool.QueryRow(ctx, `
		SELECT status, fulfilment_type FROM sales.orders
		WHERE tenant_id=$1 AND id=$2
	`, tenantID, orderID).Scan(&status, &fulfilment)
	if err != nil {
		return err
	}
	if status == "ready_for_pickup" || status == "confirmed" || status == "delivered" {
		return nil
	}
	if status != "pending_payment" {
		return fmt.Errorf("order not awaiting payment")
	}
	if err := s.inv.ConvertReservationsByReference(ctx, tenantID, "order", orderID, actorID); err != nil {
		return err
	}
	next := "confirmed"
	if fulfilment == "branch_pickup" {
		next = "ready_for_pickup"
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE sales.orders SET status=$1, updated_at=now(), version=version+1
		WHERE id=$2 AND tenant_id=$3`, next, orderID, tenantID)
	return err
}

// OnOrderPaid is invoked from the payments layer when an order payment allocates.
func (s *Service) OnOrderPaid(ctx context.Context, tenantID, orderID, actorID uuid.UUID) error {
	return s.ConfirmPaid(ctx, tenantID, orderID, actorID)
}

// CollectInBranch verifies collection code and completes BOCP handover.
func (s *Service) CollectInBranch(ctx context.Context, tenantID uuid.UUID, collectionCode string, actorID uuid.UUID) (*Order, error) {
	code := strings.TrimSpace(strings.ToUpper(collectionCode))
	var id uuid.UUID
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT id, status FROM sales.orders WHERE tenant_id=$1 AND collection_code=$2
	`, tenantID, code).Scan(&id, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("collection code not found")
		}
		return nil, err
	}
	if status != "ready_for_pickup" && status != "confirmed" {
		return nil, fmt.Errorf("order not ready for collection")
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE sales.orders SET status='delivered', updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2
	`, id, tenantID)
	if err != nil {
		return nil, err
	}
	_ = actorID
	return s.GetOrder(ctx, tenantID, id)
}

func (s *Service) GetOrder(ctx context.Context, tenantID, orderID uuid.UUID) (*Order, error) {
	var o Order
	var branchID *uuid.UUID
	var code *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, collection_code, total::float8, branch_id, fulfilment_type, created_at
		FROM sales.orders WHERE tenant_id=$1 AND id=$2`, tenantID, orderID).
		Scan(&o.ID, &o.Status, &code, &o.Total, &branchID, &o.FulfilmentType, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	o.BranchID = branchID
	// Only expose collection code after payment (anti-leakage of unpaid codes).
	if code != nil && (o.Status == "ready_for_pickup" || o.Status == "confirmed" || o.Status == "delivered") {
		o.CollectionCode = *code
	}
	return &o, nil
}

func (s *Service) ListOrders(ctx context.Context, tenantID uuid.UUID, status string) ([]Order, error) {
	q := `
		SELECT id, status, collection_code, total::float8, branch_id, fulfilment_type, created_at
		FROM sales.orders WHERE tenant_id=$1 AND channel='online'`
	args := []any{tenantID}
	if status != "" {
		q += ` AND status=$2`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Order
	for rows.Next() {
		var o Order
		var code *string
		if err := rows.Scan(&o.ID, &o.Status, &code, &o.Total, &o.BranchID, &o.FulfilmentType, &o.CreatedAt); err != nil {
			return nil, err
		}
		if code != nil && (o.Status == "ready_for_pickup" || o.Status == "confirmed" || o.Status == "delivered") {
			o.CollectionCode = *code
		}
		items = append(items, o)
	}
	if items == nil {
		items = []Order{}
	}
	return items, nil
}

// SetProductPublished controls whether a product appears in the storefront catalog.
func (s *Service) SetProductPublished(ctx context.Context, tenantID, productID uuid.UUID, published bool) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE inventory.products SET online_visible=$1, updated_at=now()
		WHERE tenant_id=$2 AND id=$3
	`, published, tenantID, productID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("product not found")
	}
	return nil
}

func (s *Service) PublishProduct(ctx context.Context, tenantID, productID uuid.UUID) error {
	return s.SetProductPublished(ctx, tenantID, productID, true)
}

type PublicBootstrap struct {
	TenantID     uuid.UUID      `json:"tenant_id"`
	TenantName   string         `json:"tenant_name"`
	BranchID     uuid.UUID      `json:"branch_id"`
	BranchName   string         `json:"branch_name"`
	LocationID   uuid.UUID      `json:"location_id"`
	LocationName string         `json:"location_name"`
	Branches     []PublicBranch `json:"branches"`
	Paybill      string         `json:"paybill,omitempty"`
}

type PublicBranch struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`
}

func (s *Service) PublicBootstrap(ctx context.Context) (*PublicBootstrap, error) {
	var b PublicBootstrap
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.name, br.id, br.name
		FROM identity.tenants t
		JOIN identity.branches br ON br.tenant_id = t.id
		ORDER BY t.created_at, br.created_at
		LIMIT 1`).Scan(&b.TenantID, &b.TenantName, &b.BranchID, &b.BranchName)
	if err != nil {
		return nil, fmt.Errorf("no tenant/branch configured")
	}
	err = s.pool.QueryRow(ctx, `
		SELECT id, name FROM inventory.stock_locations
		WHERE tenant_id = $1 AND branch_id = $2
		ORDER BY CASE WHEN location_type = 'counter' THEN 0 WHEN location_type = 'front' THEN 1 ELSE 2 END, name
		LIMIT 1`, b.TenantID, b.BranchID).Scan(&b.LocationID, &b.LocationName)
	if err != nil {
		return nil, fmt.Errorf("no stock location configured")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT br.id, br.name, loc.id, loc.name
		FROM identity.branches br
		JOIN LATERAL (
			SELECT sl.id, sl.name
			FROM inventory.stock_locations sl
			WHERE sl.tenant_id = br.tenant_id AND sl.branch_id = br.id
			ORDER BY CASE WHEN sl.location_type = 'counter' THEN 0 WHEN sl.location_type = 'front' THEN 1 ELSE 2 END, sl.name
			LIMIT 1
		) loc ON true
		WHERE br.tenant_id = $1
		ORDER BY br.name`, b.TenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	b.Branches = make([]PublicBranch, 0)
	for rows.Next() {
		var branch PublicBranch
		if err := rows.Scan(&branch.ID, &branch.Name, &branch.LocationID, &branch.LocationName); err != nil {
			return nil, err
		}
		b.Branches = append(b.Branches, branch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(mpesa_shortcode, '') FROM payments.provider_settings
		WHERE tenant_id = $1`, b.TenantID).Scan(&b.Paybill)
	return &b, nil
}

func (s *Service) ListOnlineCatalog(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID) ([]inventory.CatalogItem, error) {
	return s.inv.ListOnlineCatalog(ctx, tenantID, locationID)
}

// ExpireHolds releases due reservations and expires unpaid online orders (same as worker).
func (s *Service) ExpireHolds(ctx context.Context, tenantID uuid.UUID) (int, error) {
	released, orderIDs, err := s.inv.ExpireDueReservations(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	for _, orderID := range orderIDs {
		_, _ = s.pool.Exec(ctx, `
			UPDATE sales.orders
			SET status = 'expired', updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND id = $2 AND status = 'pending_payment'`,
			tenantID, orderID)
		_, _ = s.pool.Exec(ctx, `
			UPDATE payments.payments p
			SET status = 'failed', updated_at = now(), version = version + 1
			FROM payments.payment_allocations a
			WHERE a.payment_id = p.id
			  AND a.payable_type = 'order' AND a.payable_id = $2
			  AND p.tenant_id = $1
			  AND p.status IN ('initiated', 'pending')`,
			tenantID, orderID)
	}
	return released, nil
}

func randomCode(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out), nil
}
