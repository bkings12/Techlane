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
	"github.com/techlane/techlane/internal/notify"
	"github.com/techlane/techlane/internal/storefrontcms"
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
	pool       *pgxpool.Pool
	inv        *inventory.Service
	storefront *storefrontcms.Service
	payments   PaymentTaker
	notifier   OrderPlacedNotifier
}

// OrderPlacedNotifier alerts shop staff/owner when a storefront order is created.
type OrderPlacedNotifier interface {
	NotifyOnlineOrderPlaced(ctx context.Context, in OnlineOrderNotify) error
}

type OnlineOrderNotify struct {
	TenantID        uuid.UUID
	BranchID        uuid.UUID
	OrderID         uuid.UUID
	Total           float64
	Currency        string
	ShopName        string
	OwnerPhone      string   // legacy single number
	OwnerPhones     []string // preferred — all shop alert numbers
	CustomerName    string
	CustomerPhone   string
	FulfilmentType  string
	CollectionCode  string
	DeliverySummary string
	ItemCount       int
}

func NewService(pool *pgxpool.Pool, inv *inventory.Service, storefront *storefrontcms.Service) *Service {
	return &Service{pool: pool, inv: inv, storefront: storefront}
}

func (s *Service) SetPaymentTaker(p PaymentTaker) {
	s.payments = p
}

func (s *Service) SetOrderPlacedNotifier(n OrderPlacedNotifier) {
	s.notifier = n
}

type CartItemInput struct {
	VariantID uuid.UUID `json:"variant_id"`
	Quantity  int       `json:"quantity"`
}

type CheckoutRequest struct {
	CustomerID           *uuid.UUID      `json:"customer_id"`
	BranchID             uuid.UUID       `json:"branch_id"`
	LocationID           uuid.UUID       `json:"location_id"`
	Items                []CartItemInput `json:"items"`
	FulfilmentType       string          `json:"fulfilment_type"`
	ReservationTTLM      int             `json:"reservation_ttl_minutes"`
	Method               string          `json:"method"`
	Phone                string          `json:"phone"`
	CustomerName         string          `json:"customer_name"`
	CustomerEmail        string          `json:"customer_email"`
	CustomerNotes        string          `json:"customer_notes"`
	DeliveryLocationID   *uuid.UUID      `json:"delivery_location_id"`
	DeliveryAddressLine1 string          `json:"delivery_address_line1"`
	DeliveryAddressLine2 string          `json:"delivery_address_line2"`
	DeliveryLandmark     string          `json:"delivery_landmark"`
	ActorID              uuid.UUID       `json:"-"`
	CorrID               uuid.UUID       `json:"-"`
}

type Order struct {
	ID                   uuid.UUID           `json:"id"`
	Status               string              `json:"status"`
	CollectionCode       string              `json:"collection_code,omitempty"`
	Total                float64             `json:"total"`
	DeliveryFee          float64             `json:"delivery_fee,omitempty"`
	BranchID             *uuid.UUID          `json:"branch_id,omitempty"`
	FulfilmentType       string              `json:"fulfilment_type,omitempty"`
	CreatedAt            time.Time           `json:"created_at,omitempty"`
	Payment              *OrderPaymentResult `json:"payment,omitempty"`
	GuestName            string              `json:"guest_name,omitempty"`
	GuestPhone           string              `json:"guest_phone,omitempty"`
	GuestEmail           string              `json:"guest_email,omitempty"`
	CustomerNotes        string              `json:"customer_notes,omitempty"`
	DeliveryLocationID   *uuid.UUID          `json:"delivery_location_id,omitempty"`
	DeliveryLocationName string              `json:"delivery_location_name,omitempty"`
	DeliveryAddressLine1 string              `json:"delivery_address_line1,omitempty"`
	DeliveryAddressLine2 string              `json:"delivery_address_line2,omitempty"`
	DeliveryLandmark     string              `json:"delivery_landmark,omitempty"`
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
	req.FulfilmentType = strings.TrimSpace(strings.ToLower(req.FulfilmentType))
	if req.FulfilmentType == "" {
		req.FulfilmentType = "branch_pickup"
	}
	if req.FulfilmentType != "branch_pickup" && req.FulfilmentType != "delivery" {
		return nil, fmt.Errorf("fulfilment_type must be branch_pickup or delivery")
	}
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	req.CustomerEmail = strings.TrimSpace(req.CustomerEmail)
	req.CustomerNotes = strings.TrimSpace(req.CustomerNotes)
	req.Phone = strings.TrimSpace(req.Phone)
	req.DeliveryAddressLine1 = strings.TrimSpace(req.DeliveryAddressLine1)
	req.DeliveryAddressLine2 = strings.TrimSpace(req.DeliveryAddressLine2)
	req.DeliveryLandmark = strings.TrimSpace(req.DeliveryLandmark)
	if req.CustomerName == "" {
		return nil, fmt.Errorf("customer_name required")
	}
	if req.Phone == "" {
		return nil, fmt.Errorf("phone required")
	}
	var deliveryFee float64
	var deliveryLocName string
	var deliveryLocID *uuid.UUID
	if req.FulfilmentType == "delivery" {
		if req.DeliveryLocationID == nil || *req.DeliveryLocationID == uuid.Nil {
			return nil, fmt.Errorf("delivery_location_id required")
		}
		loc, locErr := s.GetDeliveryLocation(ctx, tenantID, *req.DeliveryLocationID)
		if locErr != nil {
			return nil, fmt.Errorf("invalid delivery location")
		}
		if !loc.Active {
			return nil, fmt.Errorf("delivery location is not available")
		}
		deliveryFee = loc.Fee
		deliveryLocName = loc.Name
		deliveryLocID = &loc.ID
	} else {
		req.DeliveryLocationID = nil
		req.DeliveryAddressLine1 = ""
		req.DeliveryAddressLine2 = ""
		req.DeliveryLandmark = ""
	}
	switch req.Method {
	case "":
		req.Method = "mpesa_stk"
	case "mpesa_stk", "mpesa_c2b", "bank_paybill", "cash_on_pickup":
		// ok — cash_on_pickup also covers cash-on-delivery for fulfilment=delivery
	default:
		return nil, fmt.Errorf("unsupported payment method")
	}
	ttl := req.ReservationTTLM
	if ttl <= 0 {
		if req.Method == "cash_on_pickup" || req.FulfilmentType == "delivery" {
			ttl = 72 * 60 // hold until pickup/delivery can complete
		} else {
			ttl = 15
		}
	}

	type resolvedLine struct {
		variantID     uuid.UUID
		quantity      int
		unitPrice     float64
		originalPrice *float64 // set only when a deal discounted this line
	}

	// preDiscountSubtotal is the sum of base catalog prices; chargeTotal is what
	// the customer actually pays after any active deal discounts — the two
	// differ only when a deal applied, and chargeTotal is what payment.Amount
	// and order.Total use everywhere downstream.
	var preDiscountSubtotal, discountTotal, chargeTotal float64
	orderID := uuid.New()
	var firstResID uuid.UUID
	lines := make([]resolvedLine, 0, len(req.Items))

	rollback := func() {
		_ = s.inv.ReleaseReservationsByReference(ctx, tenantID, "order", orderID)
	}

	for i, it := range req.Items {
		if it.Quantity <= 0 {
			rollback()
			return nil, fmt.Errorf("quantity must be positive")
		}
		// Price is resolved exactly once per line, here — never touched again for
		// this checkout — so payment amount and order_items always agree.
		basePrice, err := s.inv.GetVariantPrice(ctx, tenantID, it.VariantID)
		if err != nil {
			rollback()
			return nil, err
		}
		line := resolvedLine{variantID: it.VariantID, quantity: it.Quantity, unitPrice: basePrice}
		if s.storefront != nil {
			if deal, ok, dealErr := s.storefront.ActiveDealForVariant(ctx, tenantID, it.VariantID); dealErr == nil && ok && deal.DealPrice > 0 && deal.DealPrice < basePrice {
				line.unitPrice = deal.DealPrice
				original := basePrice
				line.originalPrice = &original
				discountTotal += (basePrice - deal.DealPrice) * float64(it.Quantity)
			}
		}
		preDiscountSubtotal += basePrice * float64(it.Quantity)
		chargeTotal += line.unitPrice * float64(it.Quantity)
		lines = append(lines, line)

		res, err := s.inv.ReserveInventory(ctx, tenantID, it.VariantID, req.LocationID, it.Quantity, time.Duration(ttl)*time.Minute, "order", orderID)
		if err != nil {
			rollback()
			return nil, err
		}
		if i == 0 {
			firstResID = res.ID
		}
	}

	chargeTotal += deliveryFee

	code, err := randomCode(6)
	if err != nil {
		rollback()
		return nil, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO sales.orders (
			id, tenant_id, branch_id, customer_id, channel, status, collection_code,
			fulfilment_type, subtotal, discount_total, delivery_fee, total, reservation_id,
			guest_name, guest_phone, guest_email, customer_notes,
			delivery_location_id, delivery_address_line1, delivery_address_line2, delivery_landmark
		) VALUES (
			$1,$2,$3,$4,'online','pending_payment',$5,$6,$7,$8,$9,$10,$11,
			NULLIF($12,''), NULLIF($13,''), NULLIF($14,''), NULLIF($15,''),
			$16, NULLIF($17,''), NULLIF($18,''), NULLIF($19,'')
		)
	`, orderID, tenantID, req.BranchID, req.CustomerID, code, req.FulfilmentType, preDiscountSubtotal, discountTotal, deliveryFee, chargeTotal, firstResID,
		req.CustomerName, req.Phone, req.CustomerEmail, req.CustomerNotes,
		deliveryLocID, req.DeliveryAddressLine1, req.DeliveryAddressLine2, req.DeliveryLandmark)
	if err != nil {
		rollback()
		return nil, err
	}

	for _, line := range lines {
		_, err = s.pool.Exec(ctx, `
			INSERT INTO sales.order_items (id, tenant_id, order_id, variant_id, quantity, unit_price, line_total, original_unit_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, uuid.New(), tenantID, orderID, line.variantID, line.quantity, line.unitPrice, line.unitPrice*float64(line.quantity), line.originalPrice)
		if err != nil {
			rollback()
			return nil, err
		}
	}

	order := &Order{
		ID:                   orderID,
		Status:               "pending_payment",
		CollectionCode:       code,
		Total:                chargeTotal,
		DeliveryFee:          deliveryFee,
		BranchID:             &req.BranchID,
		FulfilmentType:       req.FulfilmentType,
		CreatedAt:            time.Now().UTC(),
		GuestName:            req.CustomerName,
		GuestPhone:           req.Phone,
		GuestEmail:           req.CustomerEmail,
		CustomerNotes:        req.CustomerNotes,
		DeliveryLocationID:   deliveryLocID,
		DeliveryLocationName: deliveryLocName,
		DeliveryAddressLine1: req.DeliveryAddressLine1,
		DeliveryAddressLine2: req.DeliveryAddressLine2,
		DeliveryLandmark:     req.DeliveryLandmark,
	}

	result := &CheckoutResult{Order: order}
	if s.payments != nil && (req.Method == "mpesa_stk" || req.Method == "mpesa_c2b" || req.Method == "bank_paybill" || req.Method == "cash_on_pickup") {
		acct := "ORD-" + strings.ToUpper(orderID.String()[:8])
		pay, payErr := s.payments.TakeOrderPayment(ctx, OrderPaymentInput{
			TenantID: tenantID, BranchID: req.BranchID, OrderID: orderID,
			Method: req.Method, Amount: chargeTotal, Phone: req.Phone, AccountRef: acct,
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

	s.notifyOrderPlaced(ctx, tenantID, req, result.Order, len(req.Items))
	return result, nil
}

func (s *Service) notifyOrderPlaced(ctx context.Context, tenantID uuid.UUID, req CheckoutRequest, order *Order, itemCount int) {
	if s.notifier == nil || order == nil {
		return
	}
	shopName := "Shop"
	var phoneCandidates []string
	if s.storefront != nil {
		if settings, err := s.storefront.GetSettings(ctx, tenantID); err == nil {
			if n := strings.TrimSpace(settings.ShopDisplayName); n != "" {
				shopName = n
			}
			phoneCandidates = append(phoneCandidates, settings.ContactPhone)
		}
	}
	// Receipt letterhead + shop WhatsApp often hold the real counter numbers
	// even when storefront contact_phone was left blank.
	var receiptPhone, shopWA, branchPhone string
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(TRIM(phone), ''), '')
		FROM platform.receipt_settings WHERE tenant_id = $1`, tenantID).Scan(&receiptPhone)
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(TRIM(whatsapp_number), ''), '')
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).Scan(&shopWA)
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(TRIM(phone), ''), '')
		FROM identity.branches WHERE tenant_id = $1 AND id = $2`,
		tenantID, req.BranchID).Scan(&branchPhone)
	phoneCandidates = append(phoneCandidates, receiptPhone, shopWA, branchPhone)

	seen := map[string]struct{}{}
	var ownerPhones []string
	for _, raw := range phoneCandidates {
		for _, p := range notify.SplitPhoneList(raw) {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			ownerPhones = append(ownerPhones, p)
		}
	}

	deliverySummary := ""
	if req.FulfilmentType == "delivery" {
		parts := []string{order.DeliveryLocationName}
		if req.DeliveryAddressLine1 != "" {
			parts = append(parts, req.DeliveryAddressLine1)
		}
		if req.DeliveryAddressLine2 != "" {
			parts = append(parts, req.DeliveryAddressLine2)
		}
		if req.DeliveryLandmark != "" {
			parts = append(parts, "near "+req.DeliveryLandmark)
		}
		if order.DeliveryFee > 0 {
			parts = append(parts, fmt.Sprintf("fee KES %.0f", order.DeliveryFee))
		}
		deliverySummary = strings.Join(parts, ", ")
	}
	ownerPhone := ""
	if len(ownerPhones) > 0 {
		ownerPhone = ownerPhones[0]
	}
	_ = s.notifier.NotifyOnlineOrderPlaced(ctx, OnlineOrderNotify{
		TenantID:        tenantID,
		BranchID:        req.BranchID,
		OrderID:         order.ID,
		Total:           order.Total,
		Currency:        "KES",
		ShopName:        shopName,
		OwnerPhone:      ownerPhone,
		OwnerPhones:     ownerPhones,
		CustomerName:    req.CustomerName,
		CustomerPhone:   req.Phone,
		FulfilmentType:  req.FulfilmentType,
		CollectionCode:  order.CollectionCode,
		DeliverySummary: deliverySummary,
		ItemCount:       itemCount,
	})
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

// ManualConfirmPaid is the staff override: it refuses to mark an order paid
// unless a settled payment allocated to the order actually exists — or a
// pending cash_on_pickup / cash line that staff is collecting at the counter.
func (s *Service) ManualConfirmPaid(ctx context.Context, tenantID, orderID, actorID uuid.UUID) error {
	var hasPayment bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM payments.payment_allocations a
			JOIN payments.payments p ON p.id = a.payment_id
			WHERE p.tenant_id = $1 AND a.payable_type = 'order' AND a.payable_id = $2
			  AND p.status IN ('allocated', 'confirmed')
		)`, tenantID, orderID).Scan(&hasPayment)
	if err != nil {
		return err
	}
	if !hasPayment {
		tag, allocErr := s.pool.Exec(ctx, `
			UPDATE payments.payments p
			SET status = 'allocated', updated_at = now(), version = version + 1
			FROM payments.payment_allocations a
			WHERE a.payment_id = p.id
			  AND p.tenant_id = $1
			  AND a.payable_type = 'order' AND a.payable_id = $2
			  AND p.method IN ('cash_on_pickup', 'cash')
			  AND p.status IN ('pending')`, tenantID, orderID)
		if allocErr != nil {
			return allocErr
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("no confirmed payment is allocated to this order; record the payment first")
		}
	}
	return s.ConfirmPaid(ctx, tenantID, orderID, actorID)
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
	var guestName, guestPhone, guestEmail, notes *string
	var addr1, addr2, landmark *string
	var deliveryLocID *uuid.UUID
	var deliveryLocName *string
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.status, o.collection_code, o.total::float8, o.delivery_fee::float8, o.branch_id, o.fulfilment_type, o.created_at,
			o.guest_name, o.guest_phone, o.guest_email, o.customer_notes,
			o.delivery_location_id, dl.name,
			o.delivery_address_line1, o.delivery_address_line2, o.delivery_landmark
		FROM sales.orders o
		LEFT JOIN sales.delivery_locations dl ON dl.id = o.delivery_location_id
		WHERE o.tenant_id=$1 AND o.id=$2`, tenantID, orderID).
		Scan(&o.ID, &o.Status, &code, &o.Total, &o.DeliveryFee, &branchID, &o.FulfilmentType, &o.CreatedAt,
			&guestName, &guestPhone, &guestEmail, &notes,
			&deliveryLocID, &deliveryLocName,
			&addr1, &addr2, &landmark)
	if err != nil {
		return nil, err
	}
	o.BranchID = branchID
	o.DeliveryLocationID = deliveryLocID
	o.DeliveryLocationName = derefStr(deliveryLocName)
	o.GuestName = derefStr(guestName)
	o.GuestPhone = derefStr(guestPhone)
	o.GuestEmail = derefStr(guestEmail)
	o.CustomerNotes = derefStr(notes)
	o.DeliveryAddressLine1 = derefStr(addr1)
	o.DeliveryAddressLine2 = derefStr(addr2)
	o.DeliveryLandmark = derefStr(landmark)
	// Only expose collection code after payment (anti-leakage of unpaid codes).
	if code != nil && (o.Status == "ready_for_pickup" || o.Status == "confirmed" || o.Status == "delivered") {
		o.CollectionCode = *code
	}
	return &o, nil
}

func (s *Service) ListOrders(ctx context.Context, tenantID uuid.UUID, status string) ([]Order, error) {
	q := `
		SELECT o.id, o.status, o.collection_code, o.total::float8, o.delivery_fee::float8, o.branch_id, o.fulfilment_type, o.created_at,
			o.guest_name, o.guest_phone, o.guest_email, o.customer_notes,
			o.delivery_location_id, dl.name,
			o.delivery_address_line1, o.delivery_address_line2, o.delivery_landmark
		FROM sales.orders o
		LEFT JOIN sales.delivery_locations dl ON dl.id = o.delivery_location_id
		WHERE o.tenant_id=$1 AND o.channel='online'`
	args := []any{tenantID}
	if status != "" {
		q += ` AND o.status=$2`
		args = append(args, status)
	}
	q += ` ORDER BY o.created_at DESC LIMIT 100`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Order
	for rows.Next() {
		var o Order
		var code *string
		var guestName, guestPhone, guestEmail, notes *string
		var addr1, addr2, landmark *string
		var deliveryLocID *uuid.UUID
		var deliveryLocName *string
		if err := rows.Scan(&o.ID, &o.Status, &code, &o.Total, &o.DeliveryFee, &o.BranchID, &o.FulfilmentType, &o.CreatedAt,
			&guestName, &guestPhone, &guestEmail, &notes,
			&deliveryLocID, &deliveryLocName,
			&addr1, &addr2, &landmark); err != nil {
			return nil, err
		}
		o.DeliveryLocationID = deliveryLocID
		o.DeliveryLocationName = derefStr(deliveryLocName)
		o.GuestName = derefStr(guestName)
		o.GuestPhone = derefStr(guestPhone)
		o.GuestEmail = derefStr(guestEmail)
		o.CustomerNotes = derefStr(notes)
		o.DeliveryAddressLine1 = derefStr(addr1)
		o.DeliveryAddressLine2 = derefStr(addr2)
		o.DeliveryLandmark = derefStr(landmark)
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

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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
	TenantID           uuid.UUID          `json:"tenant_id"`
	TenantName         string             `json:"tenant_name"`
	BranchID           uuid.UUID          `json:"branch_id"`
	BranchName         string             `json:"branch_name"`
	LocationID         uuid.UUID          `json:"location_id"`
	LocationName       string             `json:"location_name"`
	Branches           []PublicBranch     `json:"branches"`
	DeliveryLocations  []DeliveryLocation `json:"delivery_locations"`
	Paybill            string             `json:"paybill,omitempty"`
}

type PublicBranch struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`

	// Store-locator contact details — owner-set in Settings → Branches,
	// blank fields just don't render on the storefront.
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Hours   string `json:"hours,omitempty"`
	MapURL  string `json:"map_url,omitempty"`
}

func (s *Service) PublicBootstrap(ctx context.Context) (*PublicBootstrap, error) {
	var b PublicBootstrap
	// Single-shop system: pick the sole tenant (oldest if a legacy extra exists).
	// Default branch prefers one with a stock location and a filled address —
	// never hardcode branch names; storefront checkout always lists all branches.
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.name, br.id, br.name
		FROM identity.tenants t
		JOIN identity.branches br ON br.tenant_id = t.id
		ORDER BY t.created_at,
		         (SELECT COUNT(*) FROM inventory.stock_locations sl WHERE sl.branch_id = br.id) = 0,
		         (COALESCE(NULLIF(TRIM(br.location), ''), '') = ''),
		         br.created_at DESC, br.id
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
		SELECT br.id, br.name, loc.id, loc.name,
		       COALESCE(br.location, ''), COALESCE(br.phone, ''), COALESCE(br.hours, ''), COALESCE(br.map_url, '')
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
		if err := rows.Scan(&branch.ID, &branch.Name, &branch.LocationID, &branch.LocationName,
			&branch.Address, &branch.Phone, &branch.Hours, &branch.MapURL); err != nil {
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
	locs, locErr := s.ListDeliveryLocations(ctx, b.TenantID, true)
	if locErr == nil {
		b.DeliveryLocations = locs
	} else {
		b.DeliveryLocations = []DeliveryLocation{}
	}
	return &b, nil
}

func (s *Service) ListOnlineCatalog(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID) ([]inventory.CatalogItem, error) {
	items, err := s.inv.ListOnlineCatalog(ctx, tenantID, locationID)
	if err != nil {
		return nil, err
	}
	if s.storefront == nil || len(items) == 0 {
		return items, nil
	}
	deals, err := s.storefront.ListActiveDeals(ctx, tenantID)
	if err == nil {
		byVariant := make(map[uuid.UUID]storefrontcms.Deal, len(deals))
		for _, d := range deals {
			if existing, ok := byVariant[d.VariantID]; !ok || d.DealPrice < existing.DealPrice {
				byVariant[d.VariantID] = d
			}
		}
		for i := range items {
			deal, ok := byVariant[items[i].VariantID]
			if !ok || deal.DealPrice <= 0 || deal.DealPrice >= items[i].SellPrice {
				continue
			}
			base := items[i].SellPrice
			items[i].OriginalPrice = &base
			items[i].SellPrice = deal.DealPrice
			items[i].DealEndsAt = deal.EndsAt
		}
	}
	// Deals/ratings are display overlays; a lookup failure shouldn't take the
	// whole catalog down — items already loaded above still return.

	productIDs := make([]uuid.UUID, len(items))
	for i, it := range items {
		productIDs[i] = it.ProductID
	}
	summaries, err := s.storefront.ProductRatingSummaries(ctx, tenantID, productIDs)
	if err == nil {
		for i := range items {
			if sum, ok := summaries[items[i].ProductID]; ok && sum.Count > 0 {
				avg := sum.Average
				items[i].RatingAvg = &avg
				items[i].RatingCount = sum.Count
			}
		}
	}
	return items, nil
}

// PublicStorefrontContent aggregates everything the public web-storefront's
// home page renders — settings, active banners, categories with visible
// products, merchandised product slices and resolved deals — so nothing
// there is hardcoded.
type PublicStorefrontContent struct {
	Settings    storefrontcms.Settings  `json:"settings"`
	Banners     []storefrontcms.Banner  `json:"banners"`
	Categories  []inventory.Category    `json:"categories"`
	Featured    []inventory.CatalogItem `json:"featured"`
	NewArrivals []inventory.CatalogItem `json:"new_arrivals"`
	Bestsellers []inventory.CatalogItem `json:"bestsellers"`
	Deals       []inventory.CatalogItem `json:"deals"`
	MostViewed  []inventory.CatalogItem `json:"most_viewed"`
}

func (s *Service) PublicStorefrontContent(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID) (*PublicStorefrontContent, error) {
	out := &PublicStorefrontContent{
		Settings:    storefrontcms.DefaultSettings(tenantID),
		Banners:     []storefrontcms.Banner{},
		Categories:  []inventory.Category{},
		Featured:    []inventory.CatalogItem{},
		NewArrivals: []inventory.CatalogItem{},
		Bestsellers: []inventory.CatalogItem{},
		Deals:       []inventory.CatalogItem{},
		MostViewed:  []inventory.CatalogItem{},
	}
	if s.storefront != nil {
		settings, err := s.storefront.GetSettings(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		out.Settings = settings
		banners, err := s.storefront.ListBanners(ctx, tenantID, true)
		if err != nil {
			return nil, err
		}
		out.Banners = banners
	}
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(bargain_enabled, false), COALESCE(whatsapp_number, '')
		FROM identity.shop_profiles WHERE tenant_id = $1`, tenantID).
		Scan(&out.Settings.BargainEnabled, &out.Settings.WhatsAppNumber)
	cats, err := s.inv.ListOnlineCategories(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out.Categories = cats

	items, err := s.ListOnlineCatalog(ctx, tenantID, locationID)
	if err != nil {
		return nil, err
	}
	byVariant := make(map[uuid.UUID]inventory.CatalogItem, len(items))
	for _, it := range items {
		byVariant[it.VariantID] = it
		if it.Featured && out.Settings.ShowFeatured {
			out.Featured = append(out.Featured, it)
		}
		if it.NewArrival && out.Settings.ShowNewArrivals {
			out.NewArrivals = append(out.NewArrivals, it)
		}
		if it.Bestseller && out.Settings.ShowBestsellers {
			out.Bestsellers = append(out.Bestsellers, it)
		}
		if it.OriginalPrice != nil && out.Settings.ShowDeals {
			out.Deals = append(out.Deals, it)
		}
	}

	if s.storefront != nil && out.Settings.ShowMostViewed {
		topIDs, err := s.storefront.TopViewedVariantIDs(ctx, tenantID, 12)
		if err == nil {
			for _, id := range topIDs {
				if it, ok := byVariant[id]; ok {
					out.MostViewed = append(out.MostViewed, it)
				}
			}
		}
	}
	return out, nil
}

// RecordProductView is a thin pass-through so the handler doesn't need to
// know about internal/storefrontcms directly.
func (s *Service) RecordProductView(ctx context.Context, tenantID, variantID uuid.UUID) error {
	if s.storefront == nil {
		return nil
	}
	return s.storefront.RecordView(ctx, tenantID, variantID)
}

// ListProductReviews and SubmitProductReview are thin pass-throughs.
func (s *Service) ListProductReviews(ctx context.Context, tenantID, productID uuid.UUID) ([]storefrontcms.Review, error) {
	if s.storefront == nil {
		return []storefrontcms.Review{}, nil
	}
	return s.storefront.ListProductReviews(ctx, tenantID, productID)
}

func (s *Service) SubmitProductReview(ctx context.Context, tenantID, customerID, productID uuid.UUID, rating int, title, body string) (*storefrontcms.Review, error) {
	if s.storefront == nil {
		return nil, fmt.Errorf("storefront not configured")
	}
	return s.storefront.CreateOrUpdateReview(ctx, tenantID, customerID, productID, rating, title, body)
}

// FXRates returns display-only currency conversion rates plus which
// currencies the owner has chosen to offer.
type FXRates struct {
	Base    string             `json:"base"`
	Rates   map[string]float64 `json:"rates"`
	Enabled []string           `json:"enabled"`
}

func (s *Service) FXRates(ctx context.Context, tenantID uuid.UUID) (*FXRates, error) {
	const base = "KES"
	out := &FXRates{Base: base, Rates: map[string]float64{}, Enabled: []string{}}
	if s.storefront == nil {
		return out, nil
	}
	settings, err := s.storefront.GetSettings(ctx, tenantID)
	if err == nil {
		enabled := storefrontcms.ParseEnabledCurrencies(settings.EnabledCurrencies)
		if enabled != nil {
			out.Enabled = enabled
		}
	}
	if len(out.Enabled) == 0 {
		return out, nil
	}
	rates, err := s.storefront.GetRates(ctx, base)
	if err != nil {
		return out, nil
	}
	out.Rates = rates
	return out, nil
}

// SubscribeNewsletter is a thin pass-through so the handler doesn't need to
// know about internal/storefrontcms directly.
func (s *Service) SubscribeNewsletter(ctx context.Context, tenantID uuid.UUID, email string) error {
	if s.storefront == nil {
		return fmt.Errorf("storefront not configured")
	}
	return s.storefront.Subscribe(ctx, tenantID, email)
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
