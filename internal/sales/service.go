package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryMover interface {
	GetVariantPrice(ctx context.Context, tenantID, variantID uuid.UUID) (float64, error)
	ApplyMovement(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qtyDelta int, reason, refType string, refID, actorID, corrID uuid.UUID) error
}

// AuditSink records bargained-price events. Implemented by audit.Service.
type AuditSink interface {
	Record(ctx context.Context, tenantID uuid.UUID, actorID *uuid.UUID, action, entityType string, entityID *uuid.UUID, payload map[string]any, corrID uuid.UUID) error
}

type Service struct {
	pool      *pgxpool.Pool
	inventory InventoryMover
	payments  PaymentTaker
	auditSink AuditSink
}

func NewService(pool *pgxpool.Pool, inventory InventoryMover) *Service {
	return &Service{pool: pool, inventory: inventory}
}

func (s *Service) SetAuditSink(sink AuditSink) {
	s.auditSink = sink
}

type SaleItemInput struct {
	VariantID uuid.UUID `json:"variant_id"`
	Quantity  int       `json:"quantity"`
	// Quick-sale fields: set when this line isn't in the catalog — staff sourced it
	// from a supplier on the spot. Description + UnitPrice replace the variant lookup.
	// UnitCost + SupplierID are internal-only: they generate a supplier credit entry
	// (what we owe) and let margin be computed, and are never on the customer receipt.
	Description string     `json:"description,omitempty"`
	UnitPrice   *float64   `json:"unit_price,omitempty"`
	UnitCost    *float64   `json:"unit_cost,omitempty"`
	SupplierID  *uuid.UUID `json:"supplier_id,omitempty"`
	// OverridePrice bargains a catalog line below (or above) list; requires
	// sales.price_override and a non-empty OverrideReason. List price is still
	// looked up server-side — never trusted from the client.
	OverridePrice  *float64 `json:"override_price,omitempty"`
	OverrideReason string   `json:"override_reason,omitempty"`
}

type SaleItem struct {
	// VariantID is the nil UUID for a quick-sale line — check Description instead.
	VariantID   uuid.UUID  `json:"variant_id"`
	Description string     `json:"description,omitempty"`
	Quantity    int        `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	ListPrice   *float64   `json:"list_price,omitempty"`
	LineTotal   float64    `json:"line_total"`
	UnitCost    *float64   `json:"unit_cost,omitempty"`
	SupplierID  *uuid.UUID `json:"supplier_id,omitempty"`
	Margin      *float64   `json:"margin,omitempty"`
}

type Sale struct {
	ID            uuid.UUID  `json:"id"`
	BranchID      uuid.UUID  `json:"branch_id"`
	CustomerID    *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName  string     `json:"customer_name,omitempty"`
	Channel       string     `json:"channel"`
	Status        string     `json:"status"`
	Subtotal      float64    `json:"subtotal"`
	Total         float64    `json:"total"`
	CreatedAt     time.Time  `json:"created_at"`
	PaymentMethod string     `json:"payment_method,omitempty"`
	PaymentID     *uuid.UUID `json:"payment_id,omitempty"`
	Items         []SaleItem `json:"items,omitempty"`

	// Populated only by GetSale (the details view) — ListSales stays cheap
	// for the history list, which doesn't need any of this.
	Reference        string  `json:"reference,omitempty"`
	BranchName       string  `json:"branch_name,omitempty"`
	CashierName      string  `json:"cashier_name,omitempty"`
	CustomerPhone    string  `json:"customer_phone,omitempty"`
	TaxTotal         float64 `json:"tax_total,omitempty"`
	DiscountTotal    float64 `json:"discount_total,omitempty"`
	PaidTotal        float64 `json:"paid_total,omitempty"`
	BalanceDue       float64 `json:"balance_due,omitempty"`
	PaymentStatus    string  `json:"payment_status,omitempty"`
	PaymentReference string  `json:"payment_reference,omitempty"`
}

type CreateSaleInput struct {
	TenantID           uuid.UUID
	BranchID           uuid.UUID
	CustomerID         *uuid.UUID
	Channel            string
	Items              []SaleItemInput
	ActorID            uuid.UUID
	CorrID             uuid.UUID
	AllowPriceOverride bool // set by handler after claims.HasPermission("sales.price_override")
}

func (s *Service) CreateSale(ctx context.Context, in CreateSaleInput) (*Sale, error) {
	if len(in.Items) == 0 {
		return nil, fmt.Errorf("at least one item required")
	}
	if in.Channel == "" {
		in.Channel = "pos"
	}

	var subtotal float64
	lineItems := make([]SaleItem, 0, len(in.Items))
	type priceOverrideAudit struct {
		variantID uuid.UUID
		list      float64
		charged   float64
		reason    string
	}
	var overrides []priceOverrideAudit
	for _, it := range in.Items {
		if it.Quantity <= 0 {
			return nil, fmt.Errorf("invalid quantity")
		}
		if it.VariantID != uuid.Nil {
			price, err := s.inventory.GetVariantPrice(ctx, in.TenantID, it.VariantID)
			if err != nil {
				return nil, fmt.Errorf("variant %s: %w", it.VariantID, err)
			}
			charged := price
			var listPtr *float64
			if it.OverridePrice != nil {
				if !in.AllowPriceOverride {
					return nil, fmt.Errorf("sales.price_override permission required to change catalog price")
				}
				reason := strings.TrimSpace(it.OverrideReason)
				if reason == "" {
					return nil, fmt.Errorf("override_reason required when override_price is set")
				}
				if *it.OverridePrice <= 0 {
					return nil, fmt.Errorf("override_price must be positive")
				}
				charged = *it.OverridePrice
				lp := price
				listPtr = &lp
				overrides = append(overrides, priceOverrideAudit{
					variantID: it.VariantID, list: price, charged: charged, reason: reason,
				})
			}
			line := SaleItem{
				VariantID: it.VariantID, Quantity: it.Quantity,
				UnitPrice: charged, ListPrice: listPtr,
				LineTotal: charged * float64(it.Quantity),
			}
			subtotal += line.LineTotal
			lineItems = append(lineItems, line)
			continue
		}
		// Quick sale: no catalog variant, so price and (optionally) cost/supplier are
		// supplied directly by staff instead of looked up from inventory.
		desc := strings.TrimSpace(it.Description)
		if desc == "" {
			return nil, fmt.Errorf("description required for a quick-sale item")
		}
		if it.UnitPrice == nil || *it.UnitPrice <= 0 {
			return nil, fmt.Errorf("unit_price required for a quick-sale item")
		}
		if (it.SupplierID == nil) != (it.UnitCost == nil) {
			return nil, fmt.Errorf("supplier_id and unit_cost must be provided together")
		}
		if it.UnitCost != nil && *it.UnitCost < 0 {
			return nil, fmt.Errorf("unit_cost cannot be negative")
		}
		price := *it.UnitPrice
		line := SaleItem{
			Description: desc, Quantity: it.Quantity, UnitPrice: price, LineTotal: price * float64(it.Quantity),
			UnitCost: it.UnitCost, SupplierID: it.SupplierID,
		}
		if it.UnitCost != nil {
			margin := line.LineTotal - (*it.UnitCost)*float64(it.Quantity)
			line.Margin = &margin
		}
		subtotal += line.LineTotal
		lineItems = append(lineItems, line)
	}

	id := uuid.New()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO sales.sales (id, tenant_id, branch_id, customer_id, channel, status, subtotal, total, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, 'draft', $6, $6, $7, $8)`,
		id, in.TenantID, in.BranchID, in.CustomerID, in.Channel, subtotal, in.ActorID, in.CorrID)
	if err != nil {
		return nil, err
	}
	for _, li := range lineItems {
		itemID := uuid.New()
		var variantID *uuid.UUID
		if li.VariantID != uuid.Nil {
			v := li.VariantID
			variantID = &v
		}
		var description *string
		if li.Description != "" {
			description = &li.Description
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO sales.sale_items (id, tenant_id, sale_id, variant_id, description, quantity, unit_price, list_price, line_total, unit_cost, supplier_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			itemID, in.TenantID, id, variantID, description, li.Quantity, li.UnitPrice, li.ListPrice, li.LineTotal, li.UnitCost, li.SupplierID)
		if err != nil {
			return nil, err
		}
		// Sourced from a supplier on the spot: record what we owe them in the same
		// ledger SuppliersPage's reconciliation view already reads (supplier_issue_id
		// stays NULL — that column only applies to repair part requests).
		if li.SupplierID != nil && li.UnitCost != nil {
			creditAmount := (*li.UnitCost) * float64(li.Quantity)
			note := fmt.Sprintf("Quick sale — %s", li.Description)
			_, err = tx.Exec(ctx, `
				INSERT INTO inventory.supplier_credit_entries (id, tenant_id, supplier_id, amount, entry_type, created_by, note, correlation_id)
				VALUES ($1, $2, $3, $4, 'issue', $5, $6, $7)`,
				uuid.New(), in.TenantID, *li.SupplierID, creditAmount, in.ActorID, note, in.CorrID)
			if err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.auditSink != nil {
		actor := in.ActorID
		for _, ov := range overrides {
			vid := ov.variantID
			_ = s.auditSink.Record(ctx, in.TenantID, &actor, "sales.price_override", "sale", &id, map[string]any{
				"variant_id":     vid.String(),
				"list_price":     ov.list,
				"override_price": ov.charged,
				"reason":         ov.reason,
			}, in.CorrID)
		}
	}
	return &Sale{ID: id, BranchID: in.BranchID, Channel: in.Channel, Status: "draft", Subtotal: subtotal, Total: subtotal, Items: lineItems}, nil
}

func (s *Service) CompleteSale(ctx context.Context, tenantID, saleID, locationID, actorID, corrID uuid.UUID) (*Sale, error) {
	var sale Sale
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.branch_id, s.customer_id, COALESCE(c.full_name, ''),
		       s.channel, s.status, s.subtotal, s.total
		FROM sales.sales s
		LEFT JOIN repair.customers c ON c.id = s.customer_id AND c.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.id = $2`,
		tenantID, saleID).Scan(
		&sale.ID, &sale.BranchID, &sale.CustomerID, &sale.CustomerName,
		&sale.Channel, &sale.Status, &sale.Subtotal, &sale.Total)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("sale not found")
		}
		return nil, err
	}
	if sale.Status != "draft" {
		return nil, fmt.Errorf("sale not in draft state")
	}

	// Quick-sale lines (variant_id NULL) never touched inventory, so there's no
	// stock to deduct for them.
	rows, err := s.pool.Query(ctx, `
		SELECT variant_id, quantity FROM sales.sale_items WHERE tenant_id = $1 AND sale_id = $2 AND variant_id IS NOT NULL`, tenantID, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type line struct {
		variantID uuid.UUID
		qty       int
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.variantID, &l.qty); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}

	for _, l := range lines {
		if err := s.inventory.ApplyMovement(ctx, tenantID, l.variantID, locationID, -l.qty, "sale", "sale", saleID, actorID, corrID); err != nil {
			return nil, err
		}
	}

	_, err = s.pool.Exec(ctx, `UPDATE sales.sales SET status = 'completed', updated_at = now(), version = version + 1 WHERE id = $1`, saleID)
	if err != nil {
		return nil, err
	}
	sale.Status = "completed"
	return &sale, nil
}

// CompletePaidDraftSale finishes a draft sale after its payment has allocated.
// Used by the payments STK webhook so completion does not depend on an open browser tab.
// Picks the branch counter location for stock deduction; no-ops if the sale is already past draft.
func (s *Service) CompletePaidDraftSale(ctx context.Context, tenantID, saleID, actorID uuid.UUID) error {
	var status string
	var branchID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT status, branch_id FROM sales.sales WHERE tenant_id = $1 AND id = $2`,
		tenantID, saleID).Scan(&status, &branchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if status != "draft" {
		return nil
	}
	locationID, err := s.defaultCounterLocation(ctx, tenantID, branchID)
	if err != nil {
		return err
	}
	_, err = s.CompleteSale(ctx, tenantID, saleID, locationID, actorID, uuid.New())
	if err != nil && strings.Contains(err.Error(), "not in draft") {
		return nil
	}
	return err
}

func (s *Service) defaultCounterLocation(ctx context.Context, tenantID, branchID uuid.UUID) (uuid.UUID, error) {
	var locationID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.stock_locations
		WHERE tenant_id = $1 AND location_type = 'counter'
		  AND (branch_id = $2 OR branch_id IS NULL)
		ORDER BY CASE WHEN branch_id = $2 THEN 0 ELSE 1 END, created_at
		LIMIT 1`, tenantID, branchID).Scan(&locationID)
	if err == nil {
		return locationID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.stock_locations
		WHERE tenant_id = $1
		ORDER BY CASE location_type WHEN 'counter' THEN 0 ELSE 1 END, created_at
		LIMIT 1`, tenantID).Scan(&locationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("no stock location to complete sale: %w", err)
	}
	return locationID, nil
}

func (s *Service) ReverseSale(ctx context.Context, tenantID, saleID, locationID, actorID, corrID uuid.UUID) (*Sale, error) {
	var sale Sale
	err := s.pool.QueryRow(ctx, `
		SELECT id, branch_id, channel, status, subtotal, total FROM sales.sales WHERE tenant_id = $1 AND id = $2`,
		tenantID, saleID).Scan(&sale.ID, &sale.BranchID, &sale.Channel, &sale.Status, &sale.Subtotal, &sale.Total)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("sale not found")
		}
		return nil, err
	}
	if sale.Status != "completed" {
		return nil, fmt.Errorf("only completed sales can be reversed")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT variant_id, quantity FROM sales.sale_items WHERE tenant_id = $1 AND sale_id = $2 AND variant_id IS NOT NULL`, tenantID, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var variantID uuid.UUID
		var qty int
		if err := rows.Scan(&variantID, &qty); err != nil {
			return nil, err
		}
		if err := s.inventory.ApplyMovement(ctx, tenantID, variantID, locationID, qty, "sale_reversal", "sale", saleID, actorID, corrID); err != nil {
			return nil, err
		}
	}

	_, err = s.pool.Exec(ctx, `UPDATE sales.sales SET status = 'reversed', updated_at = now(), version = version + 1 WHERE id = $1`, saleID)
	if err != nil {
		return nil, err
	}
	sale.Status = "reversed"
	return &sale, nil
}

func (s *Service) GetSale(ctx context.Context, tenantID, saleID uuid.UUID) (*Sale, error) {
	var sale Sale
	var customerPhone *string
	var createdBy *uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.branch_id, s.customer_id, COALESCE(c.full_name, ''), c.phone,
		       s.channel, s.status, s.subtotal::float8, s.tax_total::float8, s.discount_total::float8,
		       s.total::float8, s.created_at, s.created_by
		FROM sales.sales s
		LEFT JOIN repair.customers c ON c.id = s.customer_id AND c.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.id = $2`,
		tenantID, saleID).Scan(
		&sale.ID, &sale.BranchID, &sale.CustomerID, &sale.CustomerName, &customerPhone,
		&sale.Channel, &sale.Status, &sale.Subtotal, &sale.TaxTotal, &sale.DiscountTotal,
		&sale.Total, &sale.CreatedAt, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("sale not found")
		}
		return nil, err
	}
	if customerPhone != nil {
		sale.CustomerPhone = *customerPhone
	}
	sale.Reference = shortRef(sale.ID)
	_ = s.pool.QueryRow(ctx, `SELECT name FROM identity.branches WHERE tenant_id = $1 AND id = $2`,
		tenantID, sale.BranchID).Scan(&sale.BranchName)
	if createdBy != nil {
		_ = s.pool.QueryRow(ctx, `SELECT display_name FROM identity.users WHERE id = $1`, *createdBy).Scan(&sale.CashierName)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT si.variant_id, COALESCE(p.brand || ' ' || p.name, p.name, si.description, 'Item'),
		       si.quantity, si.unit_price::float8, si.line_total::float8, si.unit_cost::float8, si.supplier_id
		FROM sales.sale_items si
		LEFT JOIN inventory.product_variants v ON v.id = si.variant_id
		LEFT JOIN inventory.products p ON p.id = v.product_id
		WHERE si.tenant_id = $1 AND si.sale_id = $2
		ORDER BY si.id`, tenantID, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it SaleItem
		var variantID *uuid.UUID
		if err := rows.Scan(&variantID, &it.Description, &it.Quantity, &it.UnitPrice, &it.LineTotal, &it.UnitCost, &it.SupplierID); err != nil {
			return nil, err
		}
		if variantID != nil {
			it.VariantID = *variantID
		}
		if it.UnitCost != nil {
			margin := it.LineTotal - (*it.UnitCost)*float64(it.Quantity)
			it.Margin = &margin
		}
		sale.Items = append(sale.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	payInfos, err := s.resolveSalePayments(ctx, tenantID, saleID)
	if err != nil {
		return nil, err
	}
	for _, p := range payInfos {
		if p.Status == "allocated" || p.Status == "confirmed" {
			sale.PaidTotal += p.Amount
		}
		// The most recent payment's method/reference/status represent the sale
		// for display purposes — a sale normally has exactly one.
		sale.PaymentMethod = p.Method
		sale.PaymentStatus = p.Status
		if p.Reference != "" {
			sale.PaymentReference = p.Reference
		}
	}
	sale.BalanceDue = sale.Total - sale.PaidTotal
	if sale.BalanceDue < 0 {
		sale.BalanceDue = 0
	}
	return &sale, nil
}

// stripItemCosts removes cost/margin from a copy of the items — used by the
// HTTP handlers for a caller without reports.read. Never mutates the input.
func stripItemCosts(items []SaleItem) []SaleItem {
	out := make([]SaleItem, len(items))
	for i, it := range items {
		it.UnitCost = nil
		it.Margin = nil
		it.SupplierID = nil
		out[i] = it
	}
	return out
}

// ListSalesFilter narrows ListSales — filtering, not reporting: every field
// is a plain WHERE-clause parameter on the same query, no aggregation.
type ListSalesFilter struct {
	BranchID *uuid.UUID
	Status   string
	Method   string
	// Query matches (case-insensitively) against the sale's short reference,
	// customer name/phone, any resolved payment reference (M-Pesa/bank), or
	// any line item's product name.
	Query string
	From  *time.Time
	To    *time.Time
	Limit int
}

func (s *Service) ListSales(ctx context.Context, tenantID uuid.UUID, f ListSalesFilter) ([]Sale, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := `SELECT s.id, s.branch_id, s.customer_id, COALESCE(c.full_name, ''),
		s.channel, s.status, s.subtotal, s.total, s.created_at,
		COALESCE(pay.method, ''), pay.payment_id
		FROM sales.sales s
		LEFT JOIN repair.customers c ON c.id = s.customer_id AND c.tenant_id = s.tenant_id
		LEFT JOIN LATERAL (
			SELECT p.method, p.id AS payment_id
			FROM payments.payment_allocations a
			JOIN payments.payments p ON p.id = a.payment_id
			WHERE a.tenant_id = s.tenant_id AND a.payable_type = 'sale' AND a.payable_id = s.id
			ORDER BY p.created_at DESC
			LIMIT 1
		) pay ON true
		WHERE s.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if f.BranchID != nil {
		q += fmt.Sprintf(" AND s.branch_id = $%d", n)
		args = append(args, *f.BranchID)
		n++
	}
	if f.Status != "" {
		q += fmt.Sprintf(" AND s.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Method != "" {
		q += fmt.Sprintf(" AND pay.method = $%d", n)
		args = append(args, f.Method)
		n++
	}
	if f.From != nil {
		q += fmt.Sprintf(" AND s.created_at >= $%d", n)
		args = append(args, *f.From)
		n++
	}
	if f.To != nil {
		q += fmt.Sprintf(" AND s.created_at <= $%d", n)
		args = append(args, *f.To)
		n++
	}
	if query := strings.TrimSpace(f.Query); query != "" {
		q += fmt.Sprintf(` AND (
			s.id::text ILIKE '%%' || $%d || '%%'
			OR c.full_name ILIKE '%%' || $%d || '%%'
			OR c.phone ILIKE '%%' || $%d || '%%'
			OR EXISTS (
				SELECT 1 FROM sales.sale_items si
				LEFT JOIN inventory.product_variants v ON v.id = si.variant_id
				LEFT JOIN inventory.products p ON p.id = v.product_id
				WHERE si.tenant_id = s.tenant_id AND si.sale_id = s.id
				  AND COALESCE(p.brand || ' ' || p.name, p.name, si.description, '') ILIKE '%%' || $%d || '%%'
			)
			OR EXISTS (
				SELECT 1 FROM payments.payment_allocations a
				JOIN payments.payments pp ON pp.id = a.payment_id
				LEFT JOIN payments.mpesa_stk_transactions stk ON stk.payment_id = pp.id
				LEFT JOIN payments.mpesa_c2b_transactions c2b ON c2b.payment_id = pp.id
				LEFT JOIN payments.bank_transactions bank ON bank.payment_id = pp.id
				WHERE a.tenant_id = s.tenant_id AND a.payable_type = 'sale' AND a.payable_id = s.id
				  AND (
					COALESCE(stk.mpesa_receipt, '') ILIKE '%%' || $%d || '%%'
					OR COALESCE(c2b.trans_id, '') ILIKE '%%' || $%d || '%%'
					OR COALESCE(bank.provider_ref, '') ILIKE '%%' || $%d || '%%'
					OR COALESCE(pp.provider_ref, '') ILIKE '%%' || $%d || '%%'
				  )
			)
		)`, n, n, n, n, n, n, n, n)
		// strip an "SL-" prefix so pasting the on-screen reference still matches the raw id
		bare := strings.TrimPrefix(strings.ToLower(query), "sl-")
		args = append(args, bare)
		n++
	}
	q += fmt.Sprintf(" ORDER BY s.created_at DESC LIMIT $%d", n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Sale, 0)
	for rows.Next() {
		var sale Sale
		var paymentID *uuid.UUID
		if err := rows.Scan(
			&sale.ID, &sale.BranchID, &sale.CustomerID, &sale.CustomerName,
			&sale.Channel, &sale.Status, &sale.Subtotal, &sale.Total, &sale.CreatedAt,
			&sale.PaymentMethod, &paymentID,
		); err != nil {
			return nil, err
		}
		sale.PaymentID = paymentID
		sale.Reference = shortRef(sale.ID)
		items = append(items, sale)
	}
	return items, rows.Err()
}
