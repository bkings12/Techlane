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

type Service struct {
	pool      *pgxpool.Pool
	inventory InventoryMover
	payments  PaymentTaker
}

func NewService(pool *pgxpool.Pool, inventory InventoryMover) *Service {
	return &Service{pool: pool, inventory: inventory}
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
}

type SaleItem struct {
	// VariantID is the nil UUID for a quick-sale line — check Description instead.
	VariantID   uuid.UUID  `json:"variant_id"`
	Description string     `json:"description,omitempty"`
	Quantity    int        `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	LineTotal   float64    `json:"line_total"`
	UnitCost    *float64   `json:"unit_cost,omitempty"`
	SupplierID  *uuid.UUID `json:"supplier_id,omitempty"`
	Margin      *float64   `json:"margin,omitempty"`
}

type Sale struct {
	ID        uuid.UUID  `json:"id"`
	BranchID  uuid.UUID  `json:"branch_id"`
	Channel   string     `json:"channel"`
	Status    string     `json:"status"`
	Subtotal  float64    `json:"subtotal"`
	Total     float64    `json:"total"`
	CreatedAt time.Time  `json:"created_at"`
	Items     []SaleItem `json:"items,omitempty"`
}

type CreateSaleInput struct {
	TenantID   uuid.UUID
	BranchID   uuid.UUID
	CustomerID *uuid.UUID
	Channel    string
	Items      []SaleItemInput
	ActorID    uuid.UUID
	CorrID     uuid.UUID
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
	for _, it := range in.Items {
		if it.Quantity <= 0 {
			return nil, fmt.Errorf("invalid quantity")
		}
		if it.VariantID != uuid.Nil {
			price, err := s.inventory.GetVariantPrice(ctx, in.TenantID, it.VariantID)
			if err != nil {
				return nil, fmt.Errorf("variant %s: %w", it.VariantID, err)
			}
			line := SaleItem{VariantID: it.VariantID, Quantity: it.Quantity, UnitPrice: price, LineTotal: price * float64(it.Quantity)}
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
			INSERT INTO sales.sale_items (id, tenant_id, sale_id, variant_id, description, quantity, unit_price, line_total, unit_cost, supplier_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			itemID, in.TenantID, id, variantID, description, li.Quantity, li.UnitPrice, li.LineTotal, li.UnitCost, li.SupplierID)
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
	return &Sale{ID: id, BranchID: in.BranchID, Channel: in.Channel, Status: "draft", Subtotal: subtotal, Total: subtotal, Items: lineItems}, nil
}

func (s *Service) CompleteSale(ctx context.Context, tenantID, saleID, locationID, actorID, corrID uuid.UUID) (*Sale, error) {
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
	err := s.pool.QueryRow(ctx, `
		SELECT id, branch_id, channel, status, subtotal, total, created_at FROM sales.sales WHERE tenant_id = $1 AND id = $2`,
		tenantID, saleID).Scan(&sale.ID, &sale.BranchID, &sale.Channel, &sale.Status, &sale.Subtotal, &sale.Total, &sale.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("sale not found")
		}
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT variant_id, COALESCE(description, ''), quantity, unit_price, line_total, unit_cost, supplier_id
		FROM sales.sale_items WHERE tenant_id = $1 AND sale_id = $2`, tenantID, saleID)
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
	return &sale, nil
}

func (s *Service) ListSales(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]Sale, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := `SELECT id, branch_id, channel, status, subtotal, total, created_at
		FROM sales.sales WHERE tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if branchID != nil {
		q += fmt.Sprintf(" AND branch_id = $%d", n)
		args = append(args, *branchID)
		n++
	}
	if status != "" {
		q += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, status)
		n++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Sale, 0)
	for rows.Next() {
		var sale Sale
		if err := rows.Scan(&sale.ID, &sale.BranchID, &sale.Channel, &sale.Status, &sale.Subtotal, &sale.Total, &sale.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, sale)
	}
	return items, rows.Err()
}
