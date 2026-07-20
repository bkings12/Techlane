package sales

import (
	"context"
	"errors"
	"fmt"
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
}

type SaleItem struct {
	VariantID uuid.UUID `json:"variant_id"`
	Quantity  int       `json:"quantity"`
	UnitPrice float64   `json:"unit_price"`
	LineTotal float64   `json:"line_total"`
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
		price, err := s.inventory.GetVariantPrice(ctx, in.TenantID, it.VariantID)
		if err != nil {
			return nil, fmt.Errorf("variant %s: %w", it.VariantID, err)
		}
		line := SaleItem{VariantID: it.VariantID, Quantity: it.Quantity, UnitPrice: price, LineTotal: price * float64(it.Quantity)}
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
		_, err = tx.Exec(ctx, `
			INSERT INTO sales.sale_items (id, tenant_id, sale_id, variant_id, quantity, unit_price, line_total)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			itemID, in.TenantID, id, li.VariantID, li.Quantity, li.UnitPrice, li.LineTotal)
		if err != nil {
			return nil, err
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

	rows, err := s.pool.Query(ctx, `
		SELECT variant_id, quantity FROM sales.sale_items WHERE tenant_id = $1 AND sale_id = $2`, tenantID, saleID)
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
		SELECT variant_id, quantity FROM sales.sale_items WHERE tenant_id = $1 AND sale_id = $2`, tenantID, saleID)
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
		SELECT variant_id, quantity, unit_price, line_total FROM sales.sale_items WHERE tenant_id = $1 AND sale_id = $2`, tenantID, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it SaleItem
		if err := rows.Scan(&it.VariantID, &it.Quantity, &it.UnitPrice, &it.LineTotal); err != nil {
			return nil, err
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
