// Package search powers the web-ops command palette (Cmd+K): a single
// endpoint that fans out to customers, repair jobs, and online orders so
// staff don't have to know which page something lives on.
package search

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Result struct {
	Type     string `json:"type"` // customer | repair | order
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	URL      string `json:"url"`
}

var nonDigits = regexp.MustCompile(`[^0-9]`)

func (s *Service) Search(ctx context.Context, tenantID uuid.UUID, q string, limitPerType int) ([]Result, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Result{}, nil
	}
	if limitPerType <= 0 || limitPerType > 20 {
		limitPerType = 6
	}
	phoneDigits := nonDigits.ReplaceAllString(q, "")

	results := make([]Result, 0, limitPerType*3)

	custRows, err := s.pool.Query(ctx, `
		SELECT id, full_name, phone FROM repair.customers
		WHERE tenant_id = $1
		  AND (full_name ILIKE '%' || $2 || '%'
		       OR ($3 <> '' AND regexp_replace(COALESCE(phone, ''), '[^0-9]', '', 'g') LIKE '%' || $3 || '%'))
		ORDER BY created_at DESC LIMIT $4`, tenantID, q, phoneDigits, limitPerType)
	if err != nil {
		return nil, err
	}
	for custRows.Next() {
		var id uuid.UUID
		var name string
		var phone *string
		if err := custRows.Scan(&id, &name, &phone); err != nil {
			custRows.Close()
			return nil, err
		}
		subtitle := "Customer"
		if phone != nil && *phone != "" {
			subtitle = "Customer · " + *phone
		}
		results = append(results, Result{Type: "customer", ID: id.String(), Title: name, Subtitle: subtitle, URL: "/customers/" + id.String()})
	}
	if err := custRows.Err(); err != nil {
		custRows.Close()
		return nil, err
	}
	custRows.Close()

	repairRows, err := s.pool.Query(ctx, `
		SELECT j.id, j.job_code, j.status, COALESCE(d.brand, ''), COALESCE(d.model, ''), COALESCE(c.full_name, 'Walk-in')
		FROM repair.repair_jobs j
		LEFT JOIN repair.devices d ON d.id = j.device_id
		LEFT JOIN repair.customers c ON c.id = j.customer_id
		WHERE j.tenant_id = $1
		  AND (j.job_code ILIKE '%' || $2 || '%'
		       OR d.brand ILIKE '%' || $2 || '%'
		       OR d.model ILIKE '%' || $2 || '%'
		       OR COALESCE(d.imei, '') ILIKE '%' || $2 || '%'
		       OR COALESCE(d.serial_number, '') ILIKE '%' || $2 || '%'
		       OR c.full_name ILIKE '%' || $2 || '%')
		ORDER BY j.created_at DESC LIMIT $3`, tenantID, q, limitPerType)
	if err != nil {
		return nil, err
	}
	for repairRows.Next() {
		var id uuid.UUID
		var jobCode, status, brand, model, customerName string
		if err := repairRows.Scan(&id, &jobCode, &status, &brand, &model, &customerName); err != nil {
			repairRows.Close()
			return nil, err
		}
		device := strings.TrimSpace(brand + " " + model)
		if device == "" {
			device = "Device"
		}
		results = append(results, Result{
			Type:     "repair",
			ID:       id.String(),
			Title:    jobCode + " · " + device,
			Subtitle: customerName + " · " + status,
			URL:      "/repairs/" + id.String(),
		})
	}
	if err := repairRows.Err(); err != nil {
		repairRows.Close()
		return nil, err
	}
	repairRows.Close()

	stockRows, err := s.pool.Query(ctx, `
		SELECT v.id, p.name, v.sku, COALESCE(v.barcode, ''), COALESCE(v.sell_price, 0)::float8
		FROM inventory.product_variants v
		JOIN inventory.products p ON p.id = v.product_id
		WHERE v.tenant_id = $1
		  AND (p.name ILIKE '%' || $2 || '%' OR v.sku ILIKE '%' || $2 || '%' OR COALESCE(v.barcode, '') ILIKE '%' || $2 || '%')
		ORDER BY p.name LIMIT $3`, tenantID, q, limitPerType)
	if err != nil {
		return nil, err
	}
	for stockRows.Next() {
		var id uuid.UUID
		var name, sku, barcode string
		var price float64
		if err := stockRows.Scan(&id, &name, &sku, &barcode, &price); err != nil {
			stockRows.Close()
			return nil, err
		}
		subtitle := "SKU " + sku
		if barcode != "" {
			subtitle += " · " + barcode
		}
		results = append(results, Result{Type: "stock", ID: id.String(), Title: name, Subtitle: subtitle, URL: "/inventory"})
	}
	if err := stockRows.Err(); err != nil {
		stockRows.Close()
		return nil, err
	}
	stockRows.Close()

	orderRows, err := s.pool.Query(ctx, `
		SELECT id, status, COALESCE(collection_code, ''), total::float8
		FROM sales.orders
		WHERE tenant_id = $1 AND channel = 'online'
		  AND collection_code ILIKE '%' || $2 || '%'
		ORDER BY created_at DESC LIMIT $3`, tenantID, q, limitPerType)
	if err != nil {
		return nil, err
	}
	for orderRows.Next() {
		var id uuid.UUID
		var status, code string
		var total float64
		if err := orderRows.Scan(&id, &status, &code, &total); err != nil {
			orderRows.Close()
			return nil, err
		}
		title := "Order"
		if code != "" {
			title = "Order " + code
		}
		results = append(results, Result{
			Type:     "order",
			ID:       id.String(),
			Title:    title,
			Subtitle: status,
			URL:      "/orders",
		})
	}
	if err := orderRows.Err(); err != nil {
		orderRows.Close()
		return nil, err
	}
	orderRows.Close()

	return results, nil
}
