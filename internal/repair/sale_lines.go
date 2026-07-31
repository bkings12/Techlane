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

// JobSaleLine is an accessory (or other stock item) sold with a repair job.
type JobSaleLine struct {
	ID          uuid.UUID  `json:"id"`
	RepairJobID uuid.UUID  `json:"repair_job_id"`
	VariantID   uuid.UUID  `json:"variant_id"`
	LocationID  uuid.UUID  `json:"location_id"`
	Description string     `json:"description"`
	Quantity    int        `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	LineTotal   float64    `json:"line_total"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
}

// StockDeductor pulls sellable stock off the shelf when an accessory is added
// to a repair. Implemented by inventory without importing this package.
type StockDeductor interface {
	DeductStock(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qty int, reason, refType string, refID, actorID, corrID uuid.UUID) error
	Restock(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qty int, reason, refType string, refID, actorID, corrID uuid.UUID) error
}

func (s *Service) SetStockDeductor(d StockDeductor) {
	s.stock = d
}

// AddJobSaleLine sells a catalog item onto a repair so it joins the same
// customer balance (one cash / STK covers repair + accessories).
func (s *Service) AddJobSaleLine(
	ctx context.Context,
	tenantID, repairID, variantID, locationID uuid.UUID,
	quantity int,
	actorID, corrID uuid.UUID,
) (*JobSaleLine, error) {
	if quantity <= 0 {
		quantity = 1
	}
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, repairID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("repair not found")
	}
	if err != nil {
		return nil, err
	}
	if status == StatusCollected {
		return nil, fmt.Errorf("device already collected — cannot add accessories")
	}

	var sellPrice, costPrice float64
	var sku, productName string
	err = s.pool.QueryRow(ctx, `
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
	if sellPrice < 0 {
		return nil, fmt.Errorf("item has no sell price")
	}

	label := strings.TrimSpace(productName)
	if sku != "" {
		label = label + " (" + sku + ")"
	}
	if label == "" {
		label = "Accessory"
	}
	lineTotal := sellPrice * float64(quantity)
	id := uuid.New()

	if s.stock != nil {
		if err := s.stock.DeductStock(ctx, tenantID, variantID, locationID, quantity,
			"repair_sale", "job_sale_line", id, actorID, corrID); err != nil {
			if strings.Contains(err.Error(), "insufficient stock") {
				return nil, fmt.Errorf("insufficient stock at that location")
			}
			return nil, err
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO repair.job_sale_lines
			(id, tenant_id, repair_job_id, variant_id, location_id, description,
			 quantity, unit_price, line_total, unit_cost, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())`,
		id, tenantID, repairID, variantID, locationID, label,
		quantity, sellPrice, lineTotal, costPrice, actorID)
	if err != nil {
		if s.stock != nil {
			_ = s.stock.Restock(ctx, tenantID, variantID, locationID, quantity,
				"repair_sale_rollback", "job_sale_line", id, actorID, corrID)
		}
		return nil, err
	}

	_ = s.BookJobCost(ctx, tenantID, repairID, CostTypePartStock, label, quantity, costPrice,
		"job_sale_line", id, actorID)

	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events
			(id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		SELECT $1, tenant_id, id, status, $2, $3, $4
		FROM repair.repair_jobs WHERE tenant_id = $5 AND id = $6`,
		uuid.New(),
		fmt.Sprintf("Sold accessory: %s × %d @ %.0f", label, quantity, sellPrice),
		actorID, corrID, tenantID, repairID)

	out := &JobSaleLine{
		ID: id, RepairJobID: repairID, VariantID: variantID, LocationID: locationID,
		Description: label, Quantity: quantity, UnitPrice: sellPrice, LineTotal: lineTotal,
		CreatedAt: time.Now().UTC(), CreatedBy: &actorID,
	}
	return out, nil
}

// ListJobSaleLines returns accessories sold onto a repair.
func (s *Service) ListJobSaleLines(ctx context.Context, tenantID, repairID uuid.UUID) ([]JobSaleLine, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repair_job_id, variant_id, location_id, description, quantity,
		       unit_price::float8, line_total::float8, created_at, created_by
		FROM repair.job_sale_lines
		WHERE tenant_id = $1 AND repair_job_id = $2
		ORDER BY created_at`, tenantID, repairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobSaleLine
	for rows.Next() {
		var line JobSaleLine
		if err := rows.Scan(&line.ID, &line.RepairJobID, &line.VariantID, &line.LocationID,
			&line.Description, &line.Quantity, &line.UnitPrice, &line.LineTotal,
			&line.CreatedAt, &line.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

// RemoveJobSaleLine voids an accessory sale and puts stock back (before collection).
func (s *Service) RemoveJobSaleLine(ctx context.Context, tenantID, repairID, lineID, actorID, corrID uuid.UUID) error {
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
		return fmt.Errorf("device already collected — cannot remove accessories")
	}

	var variantID, locationID uuid.UUID
	var qty int
	var description string
	var unitCost float64
	err = s.pool.QueryRow(ctx, `
		SELECT variant_id, location_id, quantity, description, unit_cost::float8
		FROM repair.job_sale_lines
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`,
		tenantID, repairID, lineID).Scan(&variantID, &locationID, &qty, &description, &unitCost)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("sale line not found")
	}
	if err != nil {
		return err
	}

	tag, err := s.pool.Exec(ctx, `
		DELETE FROM repair.job_sale_lines
		WHERE tenant_id = $1 AND repair_job_id = $2 AND id = $3`, tenantID, repairID, lineID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("sale line not found")
	}

	if s.stock != nil {
		_ = s.stock.Restock(ctx, tenantID, variantID, locationID, qty,
			"repair_sale_void", "job_sale_line", lineID, actorID, corrID)
	}
	costAmount := unitCost * float64(qty)
	_, _ = s.pool.Exec(ctx, `
		DELETE FROM repair.job_costs
		WHERE tenant_id = $1 AND repair_job_id = $2 AND reference_type = 'job_sale_line' AND reference_id = $3`,
		tenantID, repairID, lineID)
	if costAmount > 0 {
		_, _ = s.pool.Exec(ctx, `
			UPDATE repair.repair_jobs
			SET parts_cost = GREATEST(0, COALESCE(parts_cost, 0) - $1), updated_at = now()
			WHERE tenant_id = $2 AND id = $3`, costAmount, tenantID, repairID)
	}

	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events
			(id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		SELECT $1, tenant_id, id, status, $2, $3, $4
		FROM repair.repair_jobs WHERE tenant_id = $5 AND id = $6`,
		uuid.New(), "Removed accessory sale: "+description, actorID, corrID, tenantID, repairID)
	return nil
}

func (s *Service) saleLinesTotal(ctx context.Context, tenantID, repairID uuid.UUID) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(line_total), 0)::float8
		FROM repair.job_sale_lines
		WHERE tenant_id = $1 AND repair_job_id = $2`, tenantID, repairID).Scan(&total)
	return total, err
}
