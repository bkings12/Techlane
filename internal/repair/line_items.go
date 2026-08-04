package repair

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Work-order line-item types. Explicit classification stored on the row —
// never inferred from a description/product name.
const (
	LineTypeLabour  = "labour"
	LineTypePart    = "part"
	LineTypeProduct = "product"
)

// Where a PART line came from.
const (
	PartSourceInventory = "inventory"
	PartSourceSourced   = "sourced"
)

// Procurement/usage state for a PART line. Distinct from job status — a job
// can be "waiting_parts" while its parts individually track through these.
const (
	PartStatusRequired  = "required"
	PartStatusSourcing  = "sourcing"
	PartStatusOrdered   = "ordered"
	PartStatusReceived  = "received"
	PartStatusInstalled = "installed"
	PartStatusReturned  = "returned"
	PartStatusCancelled = "cancelled"
)

// JobLineItem is one priced component of a repair work order: labour,
// a repair part (from inventory or sourced for the job), or a retail
// product. unit_cost is snapshotted at insert and never rewritten, so a
// later change to a product's catalog cost cannot rewrite historical
// profitability.
type JobLineItem struct {
	ID                 uuid.UUID  `json:"id"`
	RepairJobID        uuid.UUID  `json:"repair_job_id"`
	LineType           string     `json:"line_type"`
	Description        string     `json:"description"`
	Quantity           float64    `json:"quantity"`
	UnitPrice          float64    `json:"unit_price"`
	UnitCost           *float64   `json:"unit_cost,omitempty"`
	DiscountAmount     float64    `json:"discount_amount"`
	LineTotal          float64    `json:"line_total"`
	VariantID          *uuid.UUID `json:"variant_id,omitempty"`
	LocationID         *uuid.UUID `json:"location_id,omitempty"`
	PartSource         *string    `json:"part_source,omitempty"`
	PartStatus         *string    `json:"part_status,omitempty"`
	SupplierName       *string    `json:"supplier_name,omitempty"`
	SupplierRef        *string    `json:"supplier_ref,omitempty"`
	ExpectedArrival    *time.Time `json:"expected_arrival,omitempty"`
	AddedToInventoryAt *time.Time `json:"added_to_inventory_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	CreatedBy          *uuid.UUID `json:"created_by,omitempty"`
}

// StripCost removes the cost basis before a response reaches a caller
// without reports.read — customers and unauthorized staff must never see it.
func (l JobLineItem) StripCost() JobLineItem {
	l.UnitCost = nil
	return l
}

func StripLineItemCosts(items []JobLineItem) []JobLineItem {
	out := make([]JobLineItem, len(items))
	for i, it := range items {
		out[i] = it.StripCost()
	}
	return out
}

func (s *Service) assertJobMutable(ctx context.Context, tenantID, repairID uuid.UUID, action string) error {
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, repairID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("repair not found")
	}
	if err != nil {
		return err
	}
	if status == StatusCollected {
		return fmt.Errorf("device already collected — cannot %s", action)
	}
	return nil
}

func (s *Service) insertLineItemAuditNote(ctx context.Context, tenantID, repairID uuid.UUID, note string, actorID, corrID uuid.UUID) {
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events
			(id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		SELECT $1, tenant_id, id, status, $2, $3, $4
		FROM repair.repair_jobs WHERE tenant_id = $5 AND id = $6`,
		uuid.New(), note, actorID, corrID, tenantID, repairID)
}

func insertLineItem(ctx context.Context, s *Service, li *JobLineItem, tenantID uuid.UUID) error {
	li.ID = uuid.New()
	li.LineTotal = li.Quantity*li.UnitPrice - li.DiscountAmount
	li.CreatedAt = time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO repair.job_line_items
			(id, tenant_id, repair_job_id, line_type, description, quantity, unit_price, unit_cost,
			 discount_amount, line_total, variant_id, location_id, part_source, part_status,
			 supplier_name, supplier_ref, expected_arrival, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		li.ID, tenantID, li.RepairJobID, li.LineType, li.Description, li.Quantity, li.UnitPrice, li.UnitCost,
		li.DiscountAmount, li.LineTotal, li.VariantID, li.LocationID, li.PartSource, li.PartStatus,
		li.SupplierName, li.SupplierRef, li.ExpectedArrival, li.CreatedBy, li.CreatedAt)
	return err
}

// AddLabourLine bills a service/repair charge with no inventory or cost basis.
func (s *Service) AddLabourLine(
	ctx context.Context,
	tenantID, repairID uuid.UUID,
	description string,
	unitPrice, quantity float64,
	actorID, corrID uuid.UUID,
) (*JobLineItem, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if unitPrice < 0 {
		return nil, fmt.Errorf("unit_price cannot be negative")
	}
	if quantity <= 0 {
		quantity = 1
	}
	if err := s.assertJobMutable(ctx, tenantID, repairID, "add a service"); err != nil {
		return nil, err
	}
	li := &JobLineItem{
		RepairJobID: repairID, LineType: LineTypeLabour, Description: description,
		Quantity: quantity, UnitPrice: unitPrice, CreatedBy: &actorID,
	}
	if err := insertLineItem(ctx, s, li, tenantID); err != nil {
		return nil, err
	}
	s.insertLineItemAuditNote(ctx, tenantID, repairID,
		fmt.Sprintf("Added service: %s × %.0f @ %.0f", description, quantity, unitPrice), actorID, corrID)
	return li, nil
}

// AddInventoryPartLine draws a repair part off existing shelf stock. Stock is
// deducted immediately, same as any other inventory-backed job line — the
// part_status is a workflow/operational marker on top of that, defaulting to
// "received" because it's already physically on hand.
func (s *Service) AddInventoryPartLine(
	ctx context.Context,
	tenantID, repairID, variantID, locationID uuid.UUID,
	quantity float64,
	unitPriceOverride *float64,
	actorID, corrID uuid.UUID,
) (*JobLineItem, error) {
	return s.addCatalogLine(ctx, tenantID, repairID, variantID, locationID, quantity, unitPriceOverride,
		LineTypePart, actorID, corrID)
}

// AddProductLine sells a retail accessory onto the job, deducting stock the
// same way a normal POS sale would.
func (s *Service) AddProductLine(
	ctx context.Context,
	tenantID, repairID, variantID, locationID uuid.UUID,
	quantity float64,
	unitPriceOverride *float64,
	actorID, corrID uuid.UUID,
) (*JobLineItem, error) {
	return s.addCatalogLine(ctx, tenantID, repairID, variantID, locationID, quantity, unitPriceOverride,
		LineTypeProduct, actorID, corrID)
}

func (s *Service) addCatalogLine(
	ctx context.Context,
	tenantID, repairID, variantID, locationID uuid.UUID,
	quantity float64,
	unitPriceOverride *float64,
	lineType string,
	actorID, corrID uuid.UUID,
) (*JobLineItem, error) {
	if quantity <= 0 {
		quantity = 1
	}
	if err := s.assertJobMutable(ctx, tenantID, repairID, "add a "+lineType); err != nil {
		return nil, err
	}

	var sellPrice, costPrice float64
	var sku, productName string
	err := s.pool.QueryRow(ctx, `
		SELECT v.sell_price::float8, COALESCE(v.cost_price, 0)::float8, v.sku, p.name
		FROM inventory.product_variants v
		JOIN inventory.products p ON p.id = v.product_id
		WHERE v.tenant_id = $1 AND v.id = $2`, tenantID, variantID).
		Scan(&sellPrice, &costPrice, &sku, &productName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("stock item not found")
	}
	if err != nil {
		return nil, err
	}
	unitPrice := sellPrice
	if unitPriceOverride != nil {
		unitPrice = *unitPriceOverride
	}
	if unitPrice < 0 {
		return nil, fmt.Errorf("unit_price cannot be negative")
	}

	label := strings.TrimSpace(productName)
	if sku != "" {
		label = label + " (" + sku + ")"
	}
	if label == "" {
		label = "Item"
	}

	li := &JobLineItem{
		RepairJobID: repairID, LineType: lineType, Description: label,
		Quantity: quantity, UnitPrice: unitPrice, UnitCost: &costPrice,
		VariantID: &variantID, LocationID: &locationID, CreatedBy: &actorID,
	}
	if lineType == LineTypePart {
		src := PartSourceInventory
		status := PartStatusReceived
		li.PartSource = &src
		li.PartStatus = &status
	}

	reason := "repair_product"
	if lineType == LineTypePart {
		reason = "repair_part"
	}
	lineID := uuid.New()
	if s.stock != nil {
		intQty := int(quantity)
		if float64(intQty) != quantity || intQty <= 0 {
			intQty = 1
			if quantity > 1 {
				intQty = int(quantity + 0.5)
			}
		}
		if err := s.stock.DeductStock(ctx, tenantID, variantID, locationID, intQty,
			reason, "job_line_item", lineID, actorID, corrID); err != nil {
			if strings.Contains(err.Error(), "insufficient stock") {
				return nil, fmt.Errorf("insufficient stock at that location")
			}
			return nil, err
		}
	}
	li.ID = lineID
	li.LineTotal = li.Quantity*li.UnitPrice - li.DiscountAmount
	li.CreatedAt = time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.job_line_items
			(id, tenant_id, repair_job_id, line_type, description, quantity, unit_price, unit_cost,
			 discount_amount, line_total, variant_id, location_id, part_source, part_status, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		li.ID, tenantID, li.RepairJobID, li.LineType, li.Description, li.Quantity, li.UnitPrice, li.UnitCost,
		li.DiscountAmount, li.LineTotal, li.VariantID, li.LocationID, li.PartSource, li.PartStatus, li.CreatedBy, li.CreatedAt)
	if err != nil {
		if s.stock != nil {
			_ = s.stock.Restock(ctx, tenantID, variantID, locationID, int(quantity),
				"repair_line_rollback", "job_line_item", lineID, actorID, corrID)
		}
		return nil, err
	}

	noun := "product"
	if lineType == LineTypePart {
		noun = "part"
	}
	s.insertLineItemAuditNote(ctx, tenantID, repairID,
		fmt.Sprintf("Added %s: %s × %.0f @ %.0f", noun, label, quantity, unitPrice), actorID, corrID)
	return li, nil
}

// AddSourcedPartLine records a part procured specifically for this job —
// not drawn from and not automatically added to shop inventory.
func (s *Service) AddSourcedPartLine(
	ctx context.Context,
	tenantID, repairID uuid.UUID,
	description string,
	supplierName, supplierRef *string,
	unitCost, unitPrice, quantity float64,
	expectedArrival *time.Time,
	actorID, corrID uuid.UUID,
) (*JobLineItem, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if unitCost < 0 || unitPrice < 0 {
		return nil, fmt.Errorf("cost and price cannot be negative")
	}
	if quantity <= 0 {
		quantity = 1
	}
	if err := s.assertJobMutable(ctx, tenantID, repairID, "add a sourced part"); err != nil {
		return nil, err
	}
	source := PartSourceSourced
	status := PartStatusRequired
	li := &JobLineItem{
		RepairJobID: repairID, LineType: LineTypePart, Description: description,
		Quantity: quantity, UnitPrice: unitPrice, UnitCost: &unitCost,
		PartSource: &source, PartStatus: &status,
		SupplierName: supplierName, SupplierRef: supplierRef, ExpectedArrival: expectedArrival,
		CreatedBy: &actorID,
	}
	if err := insertLineItem(ctx, s, li, tenantID); err != nil {
		return nil, err
	}
	supplier := ""
	if supplierName != nil {
		supplier = " from " + *supplierName
	}
	s.insertLineItemAuditNote(ctx, tenantID, repairID,
		fmt.Sprintf("Sourced part: %s × %.0f @ %.0f (cost %.0f)%s", description, quantity, unitPrice, unitCost, supplier),
		actorID, corrID)
	return li, nil
}

// RemoveLineItem deletes a line item, reversing any stock it deducted.
func (s *Service) RemoveLineItem(ctx context.Context, tenantID, repairID, lineID, actorID, corrID uuid.UUID) error {
	if err := s.assertJobMutable(ctx, tenantID, repairID, "remove a line item"); err != nil {
		return err
	}
	var lineType, description string
	var variantID, locationID *uuid.UUID
	var qty float64
	err := s.pool.QueryRow(ctx, `
		SELECT line_type, description, variant_id, location_id, quantity
		FROM repair.job_line_items
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, lineID).Scan(&lineType, &description, &variantID, &locationID, &qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("line item not found")
	}
	if err != nil {
		return err
	}

	tag, err := s.pool.Exec(ctx, `
		DELETE FROM repair.job_line_items
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`, tenantID, repairID, lineID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("line item not found")
	}

	if s.stock != nil && variantID != nil && locationID != nil {
		_ = s.stock.Restock(ctx, tenantID, *variantID, *locationID, int(qty),
			"repair_line_void", "job_line_item", lineID, actorID, corrID)
	}

	s.insertLineItemAuditNote(ctx, tenantID, repairID,
		fmt.Sprintf("Removed %s: %s", lineType, description), actorID, corrID)
	return nil
}

// UpdateLineItem lets staff correct a price or move a part through its
// procurement workflow after it's been added. Both are audited.
func (s *Service) UpdateLineItem(
	ctx context.Context,
	tenantID, repairID, lineID uuid.UUID,
	unitPrice *float64,
	partStatus *string,
	actorID, corrID uuid.UUID,
) (*JobLineItem, error) {
	if err := s.assertJobMutable(ctx, tenantID, repairID, "update a line item"); err != nil {
		return nil, err
	}
	var lineType, description string
	var oldPrice, qty, discount float64
	var oldStatus *string
	err := s.pool.QueryRow(ctx, `
		SELECT line_type, description, unit_price::float8, quantity::float8, discount_amount::float8, part_status
		FROM repair.job_line_items
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, lineID).Scan(&lineType, &description, &oldPrice, &qty, &discount, &oldStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("line item not found")
	}
	if err != nil {
		return nil, err
	}

	newPrice := oldPrice
	if unitPrice != nil {
		if *unitPrice < 0 {
			return nil, fmt.Errorf("unit_price cannot be negative")
		}
		newPrice = *unitPrice
	}
	newStatus := oldStatus
	if partStatus != nil {
		if lineType != LineTypePart {
			return nil, fmt.Errorf("part_status only applies to part lines")
		}
		newStatus = partStatus
	}
	lineTotal := qty*newPrice - discount
	_, err = s.pool.Exec(ctx, `
		UPDATE repair.job_line_items
		SET unit_price = $1, line_total = $2, part_status = $3
		WHERE tenant_id = $4 AND repair_job_id = $5 AND id = $6`,
		newPrice, lineTotal, newStatus, tenantID, repairID, lineID)
	if err != nil {
		return nil, err
	}

	if unitPrice != nil && newPrice != oldPrice {
		s.insertLineItemAuditNote(ctx, tenantID, repairID,
			fmt.Sprintf("Price changed on %s: %.0f → %.0f", description, oldPrice, newPrice), actorID, corrID)
	}
	if partStatus != nil && (oldStatus == nil || *oldStatus != *partStatus) {
		from := "required"
		if oldStatus != nil {
			from = *oldStatus
		}
		s.insertLineItemAuditNote(ctx, tenantID, repairID,
			fmt.Sprintf("Part status on %s: %s → %s", description, from, *partStatus), actorID, corrID)
	}
	return s.getLineItem(ctx, tenantID, repairID, lineID)
}

// AddSourcedPartToInventory opts a sourced part's remaining/received stock
// into normal inventory — an explicit, separate action so sourcing a part for
// one job never silently pollutes retail stock. Requires an existing
// product/variant to receive against.
func (s *Service) AddSourcedPartToInventory(
	ctx context.Context,
	tenantID, repairID, lineID, variantID uuid.UUID,
	quantity float64,
	actorID, corrID uuid.UUID,
) error {
	if quantity <= 0 {
		return fmt.Errorf("quantity must be greater than zero")
	}
	var lineType string
	var partSource *string
	err := s.pool.QueryRow(ctx, `
		SELECT line_type, part_source FROM repair.job_line_items
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, lineID).Scan(&lineType, &partSource)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("line item not found")
	}
	if err != nil {
		return err
	}
	if lineType != LineTypePart || partSource == nil || *partSource != PartSourceSourced {
		return fmt.Errorf("only a sourced part line can be added to inventory")
	}
	var locationID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.stock_locations WHERE tenant_id = $1 ORDER BY created_at LIMIT 1`,
		tenantID).Scan(&locationID)
	if err != nil {
		return fmt.Errorf("no stock location available: %w", err)
	}
	if s.stock == nil {
		return fmt.Errorf("inventory not available")
	}
	if err := s.stock.Restock(ctx, tenantID, variantID, locationID, int(quantity),
		"sourced_part_surplus", "job_line_item", lineID, actorID, corrID); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE repair.job_line_items SET added_to_inventory_at = now()
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`, tenantID, repairID, lineID)
	if err != nil {
		return err
	}
	s.insertLineItemAuditNote(ctx, tenantID, repairID,
		fmt.Sprintf("Added %.0f remaining unit(s) to inventory", quantity), actorID, corrID)
	return nil
}

func (s *Service) getLineItem(ctx context.Context, tenantID, repairID, lineID uuid.UUID) (*JobLineItem, error) {
	items, err := s.JobLineItems(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.ID == lineID {
			return &it, nil
		}
	}
	return nil, fmt.Errorf("line item not found")
}

// JobLineItems returns every labour/part/product line on a job, oldest first.
func (s *Service) JobLineItems(ctx context.Context, tenantID, repairID uuid.UUID) ([]JobLineItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repair_job_id, line_type, description, quantity::float8, unit_price::float8, unit_cost::float8,
		       discount_amount::float8, line_total::float8, variant_id, location_id, part_source, part_status,
		       supplier_name, supplier_ref, expected_arrival, added_to_inventory_at, created_at, created_by
		FROM repair.job_line_items
		WHERE tenant_id = $1 AND repair_job_id = $2
		ORDER BY created_at`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]JobLineItem, 0)
	for rows.Next() {
		var it JobLineItem
		if err := rows.Scan(&it.ID, &it.RepairJobID, &it.LineType, &it.Description, &it.Quantity, &it.UnitPrice,
			&it.UnitCost, &it.DiscountAmount, &it.LineTotal, &it.VariantID, &it.LocationID, &it.PartSource,
			&it.PartStatus, &it.SupplierName, &it.SupplierRef, &it.ExpectedArrival, &it.AddedToInventoryAt,
			&it.CreatedAt, &it.CreatedBy); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// LineItemTotals is the per-type revenue/cost rollup for a single job.
type LineItemTotals struct {
	LabourTotal     float64
	HasLabourLines  bool
	PartsRevenue    float64
	PartsCost       float64
	HasPartLines    bool
	ProductsRevenue float64
	ProductsCost    float64
	HasProductLines bool
}

func (s *Service) lineItemTotals(ctx context.Context, tenantID, repairID uuid.UUID) (*LineItemTotals, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT line_type,
		       SUM(quantity*unit_price - discount_amount)::float8,
		       SUM(quantity*COALESCE(unit_cost,0))::float8
		FROM repair.job_line_items
		WHERE tenant_id = $1 AND repair_job_id = $2
		GROUP BY line_type`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t := &LineItemTotals{}
	for rows.Next() {
		var lineType string
		var revenue, cost float64
		if err := rows.Scan(&lineType, &revenue, &cost); err != nil {
			return nil, err
		}
		switch lineType {
		case LineTypeLabour:
			t.LabourTotal = revenue
			t.HasLabourLines = true
		case LineTypePart:
			t.PartsRevenue = revenue
			t.PartsCost = cost
			t.HasPartLines = true
		case LineTypeProduct:
			t.ProductsRevenue = revenue
			t.ProductsCost = cost
			t.HasProductLines = true
		}
	}
	return t, rows.Err()
}

// JobFinancials is the internal profitability breakdown for a job — revenue,
// cost, and gross profit per category. Never rendered to the customer.
type JobFinancials struct {
	LabourRevenue     float64 `json:"labour_revenue"`
	PartsRevenue      float64 `json:"parts_revenue"`
	PartsCost         float64 `json:"parts_cost"`
	PartsProfit       float64 `json:"parts_profit"`
	ProductsRevenue   float64 `json:"products_revenue"`
	ProductsCost      float64 `json:"products_cost"`
	ProductsProfit    float64 `json:"products_profit"`
	TotalRevenue      float64 `json:"total_revenue"`
	TotalCOGS         float64 `json:"total_cogs"`
	GrossProfit       float64 `json:"gross_profit"`
	UsingLegacyMargin bool    `json:"using_legacy_margin"`
}

// JobFinancialsFor computes the labour/parts/products profitability
// breakdown for a job. Falls back to the legacy labor_amount/parts_cost
// margin (JobMarginFor) for jobs that predate line items entirely.
func (s *Service) JobFinancialsFor(ctx context.Context, tenantID, repairID uuid.UUID) (*JobFinancials, error) {
	t, err := s.lineItemTotals(ctx, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	if !t.HasLabourLines && !t.HasPartLines && !t.HasProductLines {
		legacy, err := s.JobMarginFor(ctx, tenantID, repairID)
		if err != nil {
			return nil, err
		}
		return financialsFromLegacyMargin(legacy), nil
	}
	return financialsFromTotals(t), nil
}

// financialsFromLegacyMargin adapts the pre-line-item margin view (labour vs.
// parts cost only) to the JobFinancials shape, for jobs with no line items.
func financialsFromLegacyMargin(legacy *JobMargin) *JobFinancials {
	f := &JobFinancials{
		LabourRevenue:     legacy.LaborAmount,
		PartsCost:         legacy.PartsCost,
		PartsProfit:       -legacy.PartsCost,
		UsingLegacyMargin: true,
	}
	f.TotalRevenue = f.LabourRevenue
	f.TotalCOGS = f.PartsCost
	f.GrossProfit = f.TotalRevenue - f.TotalCOGS
	return f
}

// financialsFromTotals is the pure labour/parts/products → revenue/cost/profit
// computation, split out from JobFinancialsFor so it's testable without a DB.
func financialsFromTotals(t *LineItemTotals) *JobFinancials {
	f := &JobFinancials{
		LabourRevenue:   t.LabourTotal,
		PartsRevenue:    t.PartsRevenue,
		PartsCost:       t.PartsCost,
		PartsProfit:     t.PartsRevenue - t.PartsCost,
		ProductsRevenue: t.ProductsRevenue,
		ProductsCost:    t.ProductsCost,
		ProductsProfit:  t.ProductsRevenue - t.ProductsCost,
	}
	f.TotalRevenue = f.LabourRevenue + f.PartsRevenue + f.ProductsRevenue
	f.TotalCOGS = f.PartsCost + f.ProductsCost
	f.GrossProfit = f.TotalRevenue - f.TotalCOGS
	return f
}
