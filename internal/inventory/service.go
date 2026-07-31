package inventory

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
	"github.com/techlane/techlane/packages/pkg/events"
)

const authCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const authCodeLength = 8

type Service struct {
	pool        *pgxpool.Pool
	bus         *events.Bus
	alertHook   AlertHook
	jobCostHook JobCostHook
	store       ObjectStore
}

// ObjectStore is the slice of object storage inventory needs for product images.
type ObjectStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

func (s *Service) SetObjectStore(store ObjectStore) { s.store = store }

const maxImageBytes = 2 * 1024 * 1024

// AlertHook clears risk alerts when supplier issues are reconciled.
type AlertHook interface {
	ResolveOpenAlertsByEntity(ctx context.Context, tenantID uuid.UUID, kind string, entityID, resolver uuid.UUID) (int64, error)
}

// JobCostHook books the cost of a consumed part against the repair job that used
// it, so per-job margin includes parts whichever way they were sourced.
type JobCostHook interface {
	BookPartCost(ctx context.Context, tenantID, repairJobID uuid.UUID, costType, description string,
		quantity int, unitCost float64, refType string, refID, actorID uuid.UUID) error
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) SetEventBus(bus *events.Bus) {
	s.bus = bus
	if bus == nil {
		return
	}
	bus.Subscribe("repair.closed", func(e events.Envelope) {
		s.onRepairClosed(context.Background(), e)
	})
}

// onRepairClosed voids any part authorisation still outstanding on a job that has
// been cancelled or written off, so nobody can redeem an auth code for a job that
// no longer exists.
func (s *Service) onRepairClosed(ctx context.Context, e events.Envelope) {
	jobID, ok := e.Payload["repair_job_id"].(string)
	if !ok {
		return
	}
	repairJobID, err := uuid.Parse(jobID)
	if err != nil {
		return
	}
	actorID := uuid.Nil
	if e.ActorID != nil {
		actorID = *e.ActorID
	}
	status, _ := e.Payload["status"].(string)
	reason, _ := e.Payload["reason"].(string)
	label := strings.TrimSpace(status + " " + reason)
	if label == "" {
		label = "job closed"
	}
	n, err := s.CancelOutstandingPartsForJob(ctx, e.TenantID, repairJobID, actorID, "job "+label)
	if err != nil {
		s.publish("part_request.void_failed", e.TenantID, uuid.Nil, actorID, e.CorrelationID, map[string]any{
			"repair_job_id": repairJobID.String(), "error": err.Error(),
		})
		return
	}
	if n > 0 {
		s.publish("part_request.voided", e.TenantID, uuid.Nil, actorID, e.CorrelationID, map[string]any{
			"repair_job_id": repairJobID.String(), "count": n, "reason": label,
		})
	}
}

func (s *Service) SetAlertHook(h AlertHook) {
	s.alertHook = h
}

func (s *Service) SetJobCostHook(h JobCostHook) {
	s.jobCostHook = h
}

// bookPartCost is a best-effort cost booking: the part has already physically moved
// by the time we get here, so a costing failure must not roll that back. It is
// surfaced as an event instead so it can be chased.
func (s *Service) bookPartCost(ctx context.Context, tenantID, repairJobID uuid.UUID, costType, description string,
	quantity int, unitCost float64, refType string, refID, actorID uuid.UUID) {
	if s.jobCostHook == nil || repairJobID == uuid.Nil {
		return
	}
	if err := s.jobCostHook.BookPartCost(ctx, tenantID, repairJobID, costType, description,
		quantity, unitCost, refType, refID, actorID); err != nil {
		s.publish("job_cost.book_failed", tenantID, uuid.Nil, actorID, uuid.Nil, map[string]any{
			"repair_job_id": repairJobID.String(), "reference_id": refID.String(), "error": err.Error(),
		})
	}
}

func (s *Service) publish(eventType string, tenantID, branchID, actorID, corrID uuid.UUID, payload map[string]any) {
	if s.bus == nil {
		return
	}
	env := events.New(eventType, tenantID, corrID, payload)
	if branchID != uuid.Nil {
		env.BranchID = &branchID
	}
	if actorID != uuid.Nil {
		env.ActorID = &actorID
	}
	s.bus.Publish(env)
}

func (s *Service) Start(ctx context.Context, tenantID uuid.UUID) error {
	var supplierID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.suppliers WHERE tenant_id = $1 ORDER BY created_at LIMIT 1`, tenantID).Scan(&supplierID)
	if errors.Is(err, pgx.ErrNoRows) {
		supplierID = uuid.New()
		if _, err := s.pool.Exec(ctx, `INSERT INTO inventory.suppliers (id, tenant_id, name) VALUES ($1, $2, $3)`,
			supplierID, tenantID, "Default Screens"); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := s.ensureDefaultSupplierContact(ctx, tenantID, supplierID); err != nil {
		return err
	}
	return s.EnsurePOSCatalog(ctx, tenantID)
}

func (s *Service) EnsurePOSCatalog(ctx context.Context, tenantID uuid.UUID) error {
	var productCount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM inventory.products WHERE tenant_id = $1`, tenantID).Scan(&productCount); err != nil {
		return err
	}

	locID, _, err := s.EnsureStockLocations(ctx, tenantID)
	if err != nil {
		return err
	}

	if productCount > 0 {
		return nil
	}

	type seedItem struct {
		name, brand, sku string
		price            float64
		qty              int
	}
	items := []seedItem{
		{"Tempered glass", "Generic", "ACC-GLASS-UNI", 500, 50},
		{"USB-C cable", "TechLane", "ACC-CABLE-USBC", 800, 40},
		{"Phone case", "TechLane", "ACC-CASE-UNI", 1200, 30},
		{"Power bank 10000mAh", "Anker", "ACC-PB-10K", 3500, 15},
		{"Earphones", "Generic", "ACC-EAR-3.5", 900, 25},
	}
	for _, it := range items {
		pid := uuid.New()
		brand := it.brand
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO inventory.products (id, tenant_id, name, brand, pos_visible, online_visible)
			VALUES ($1, $2, $3, $4, true, true)`, pid, tenantID, it.name, brand); err != nil {
			return err
		}
		vid := uuid.New()
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price)
			VALUES ($1, $2, $3, $4, $5)`, vid, tenantID, pid, it.sku, it.price); err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO inventory.inventory_balances (id, tenant_id, variant_id, location_id, physical_qty, available_qty)
			VALUES ($1, $2, $3, $4, $5, $5)`, uuid.New(), tenantID, vid, locID, it.qty); err != nil {
			return err
		}
	}
	return nil
}

// EnsureStockLocations creates Front counter + Parts store when the tenant has none.
func (s *Service) EnsureStockLocations(ctx context.Context, tenantID uuid.UUID) (counterID, storeID uuid.UUID, err error) {
	var branchID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT id FROM identity.branches WHERE tenant_id = $1 ORDER BY created_at LIMIT 1`, tenantID).Scan(&branchID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("no branch for stock locations: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.stock_locations WHERE tenant_id = $1 AND location_type = 'counter' LIMIT 1`, tenantID).Scan(&counterID)
	if errors.Is(err, pgx.ErrNoRows) {
		counterID = uuid.New()
		_, err = s.pool.Exec(ctx, `
			INSERT INTO inventory.stock_locations (id, tenant_id, branch_id, name, location_type)
			VALUES ($1, $2, $3, 'Front counter', 'counter')`, counterID, tenantID, branchID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
	} else if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.stock_locations WHERE tenant_id = $1 AND location_type = 'store' LIMIT 1`, tenantID).Scan(&storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		storeID = uuid.New()
		_, err = s.pool.Exec(ctx, `
			INSERT INTO inventory.stock_locations (id, tenant_id, branch_id, name, location_type)
			VALUES ($1, $2, $3, 'Parts store', 'store')`, storeID, tenantID, branchID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
	} else if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return counterID, storeID, nil
}

type StockLocation struct {
	ID           uuid.UUID  `json:"id"`
	BranchID     *uuid.UUID `json:"branch_id,omitempty"`
	Name         string     `json:"name"`
	LocationType string     `json:"location_type"`
}

type CatalogItem struct {
	VariantID    uuid.UUID `json:"variant_id"`
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	Brand        *string   `json:"brand,omitempty"`
	Category     *string   `json:"category,omitempty"`
	Description  *string   `json:"description,omitempty"`
	ImageURL     *string   `json:"image_url,omitempty"`
	HasImage     bool      `json:"has_image,omitempty"`
	ImageUpdated *time.Time `json:"image_updated_at,omitempty"`
	SKU          string    `json:"sku"`
	SellPrice    float64   `json:"sell_price"`
	AvailableQty int       `json:"available_qty"`
	LocationID   uuid.UUID `json:"location_id,omitempty"`

	// Populated by internal/commerce.ListOnlineCatalog when an active deal
	// applies — SellPrice becomes the deal price and OriginalPrice carries
	// the pre-discount price for "was/now" display.
	OriginalPrice *float64   `json:"original_price,omitempty"`
	DealEndsAt    *time.Time `json:"deal_ends_at,omitempty"`

	Featured   bool `json:"featured,omitempty"`
	NewArrival bool `json:"new_arrival,omitempty"`
	Bestseller bool `json:"bestseller,omitempty"`
	SortOrder  int  `json:"sort_order,omitempty"`

	// Populated by internal/commerce.ListOnlineCatalog from published
	// platform.product_reviews — omitted entirely when no reviews exist yet,
	// rather than showing a fabricated zero-star rating.
	RatingAvg   *float64 `json:"rating_avg,omitempty"`
	RatingCount int      `json:"rating_count,omitempty"`
}

func (s *Service) ListStockLocations(ctx context.Context, tenantID uuid.UUID, branchID *uuid.UUID) ([]StockLocation, error) {
	q := `SELECT id, branch_id, name, location_type FROM inventory.stock_locations WHERE tenant_id = $1`
	args := []any{tenantID}
	if branchID != nil {
		q += ` AND (branch_id = $2 OR branch_id IS NULL)`
		args = append(args, *branchID)
	}
	q += ` ORDER BY name`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StockLocation
	for rows.Next() {
		var loc StockLocation
		if err := rows.Scan(&loc.ID, &loc.BranchID, &loc.Name, &loc.LocationType); err != nil {
			return nil, err
		}
		items = append(items, loc)
	}
	if items == nil {
		items = []StockLocation{}
	}
	return items, nil
}

func (s *Service) ListPOSCatalog(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID) ([]CatalogItem, error) {
	q := `
		SELECT v.id, v.product_id, p.name, p.brand, p.category, p.description, p.image_url, v.sku, v.sell_price::float8,
		       COALESCE(b.available_qty, 0), COALESCE(b.location_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM inventory.product_variants v
		JOIN inventory.products p ON p.id = v.product_id
		LEFT JOIN inventory.inventory_balances b ON b.variant_id = v.id AND b.tenant_id = v.tenant_id`
	args := []any{tenantID}
	n := 2
	if locationID != nil {
		q += fmt.Sprintf(` AND b.location_id = $%d`, n)
		args = append(args, *locationID)
		n++
	}
	q += `
		WHERE v.tenant_id = $1 AND COALESCE(p.pos_visible, true) = true
		ORDER BY p.name, v.sku`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CatalogItem
	for rows.Next() {
		var it CatalogItem
		var loc uuid.UUID
		if err := rows.Scan(&it.VariantID, &it.ProductID, &it.ProductName, &it.Brand, &it.Category, &it.Description, &it.ImageURL, &it.SKU, &it.SellPrice, &it.AvailableQty, &loc); err != nil {
			return nil, err
		}
		if loc != uuid.Nil {
			it.LocationID = loc
		}
		items = append(items, it)
	}
	if items == nil {
		items = []CatalogItem{}
	}
	return items, nil
}

func (s *Service) ListOnlineCatalog(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID) ([]CatalogItem, error) {
	q := `
		SELECT v.id, v.product_id, p.name, p.brand, p.category, p.description, p.image_url, v.sku, v.sell_price::float8,
		       COALESCE(b.available_qty, 0), COALESCE(b.location_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       p.featured, p.new_arrival, p.bestseller, p.storefront_sort_order,
		       (p.image_object_key IS NOT NULL OR p.image_bytes IS NOT NULL), p.image_updated_at
		FROM inventory.product_variants v
		JOIN inventory.products p ON p.id = v.product_id
		LEFT JOIN inventory.inventory_balances b ON b.variant_id = v.id AND b.tenant_id = v.tenant_id`
	args := []any{tenantID}
	n := 2
	if locationID != nil {
		q += fmt.Sprintf(` AND b.location_id = $%d`, n)
		args = append(args, *locationID)
		n++
	}
	q += `
		WHERE v.tenant_id = $1 AND COALESCE(p.online_visible, false) = true
		ORDER BY p.storefront_sort_order, p.name, v.sku`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CatalogItem
	for rows.Next() {
		var it CatalogItem
		var loc uuid.UUID
		if err := rows.Scan(&it.VariantID, &it.ProductID, &it.ProductName, &it.Brand, &it.Category, &it.Description, &it.ImageURL, &it.SKU, &it.SellPrice, &it.AvailableQty, &loc,
			&it.Featured, &it.NewArrival, &it.Bestseller, &it.SortOrder, &it.HasImage, &it.ImageUpdated); err != nil {
			return nil, err
		}
		if loc != uuid.Nil {
			it.LocationID = loc
		}
		// Uploaded product photos take precedence over a free-text image_url.
		if it.HasImage {
			it.ImageURL = nil
		}
		items = append(items, it)
	}
	if items == nil {
		items = []CatalogItem{}
	}
	return items, nil
}

func GenerateAuthCode() (string, error) {
	out := make([]byte, authCodeLength)
	max := big.NewInt(int64(len(authCodeChars)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = authCodeChars[n.Int64()]
	}
	return string(out), nil
}

type PartRequest struct {
	ID                 uuid.UUID          `json:"id"`
	RepairJobID        uuid.UUID          `json:"repair_job_id"`
	JobCode            string             `json:"job_code,omitempty"`
	Status             string             `json:"status"`
	Description        string             `json:"description"`
	Quantity           int                `json:"quantity"`
	AssignedSupplierID *uuid.UUID         `json:"assigned_supplier_id,omitempty"`
	QuoteStatus        *string            `json:"quote_status,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	Quotes             []PartRequestQuote `json:"quotes,omitempty"`
	Issue              *SupplierIssue     `json:"issue,omitempty"`
}

func (s *Service) ListPartRequestsForRepair(ctx context.Context, tenantID, repairJobID uuid.UUID) ([]PartRequest, error) {
	return s.ListPartRequests(ctx, tenantID, &repairJobID, "")
}

func (s *Service) ListPartRequests(ctx context.Context, tenantID uuid.UUID, repairJobID *uuid.UUID, status string) ([]PartRequest, error) {
	q := `
		SELECT pr.id, pr.repair_job_id, rj.job_code, pr.status, pr.description, pr.quantity, pr.created_at,
		       pr.assigned_supplier_id, pr.quote_status,
		       si.id, si.supplier_id, si.auth_code, si.status, si.unit_cost, si.collected_at, si.reconciliation_status
		FROM inventory.part_requests pr
		JOIN repair.repair_jobs rj ON rj.id = pr.repair_job_id
		LEFT JOIN inventory.supplier_issues si ON si.part_request_id = pr.id
		WHERE pr.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if repairJobID != nil {
		q += fmt.Sprintf(" AND pr.repair_job_id = $%d", n)
		args = append(args, *repairJobID)
		n++
	}
	if status != "" {
		q += fmt.Sprintf(" AND pr.status = $%d", n)
		args = append(args, status)
	}
	q += " ORDER BY pr.created_at DESC LIMIT 200"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var items []PartRequest
	for rows.Next() {
		var pr PartRequest
		var issueID, supplierID *uuid.UUID
		var authCode, issueStatus, recon *string
		var unitCost *float64
		var collectedAt *time.Time
		if err := rows.Scan(
			&pr.ID, &pr.RepairJobID, &pr.JobCode, &pr.Status, &pr.Description, &pr.Quantity, &pr.CreatedAt,
			&pr.AssignedSupplierID, &pr.QuoteStatus,
			&issueID, &supplierID, &authCode, &issueStatus, &unitCost, &collectedAt, &recon,
		); err != nil {
			return nil, err
		}
		if issueID != nil {
			si := SupplierIssue{
				ID: *issueID, PartRequestID: pr.ID, RepairJobID: pr.RepairJobID,
				CollectedAt: collectedAt,
			}
			if supplierID != nil {
				si.SupplierID = *supplierID
			}
			if authCode != nil {
				si.AuthCode = *authCode
			}
			if issueStatus != nil {
				si.Status = *issueStatus
			}
			if unitCost != nil {
				si.UnitCost = *unitCost
			}
			if recon != nil {
				si.ReconciliationStatus = *recon
			}
			pr.Issue = &si
		}
		items = append(items, pr)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range items {
		quotes, err := s.listQuotesForRequest(ctx, tenantID, items[i].ID, nil)
		if err != nil {
			return nil, err
		}
		items[i].Quotes = quotes
		// Promote leftover pending quotes (from before auto-authorize) into an auth code
		// so the shop never sees a stuck "waiting" state with a price already set.
		s.finalizeStuckPendingQuote(ctx, tenantID, &items[i])
		items[i].Quotes = filterActiveQuotes(items[i].Quotes)
	}
	return items, nil
}

func filterActiveQuotes(quotes []PartRequestQuote) []PartRequestQuote {
	out := make([]PartRequestQuote, 0, len(quotes))
	for _, q := range quotes {
		if q.Status == "superseded" || q.Status == "declined" {
			continue
		}
		out = append(out, q)
	}
	return out
}

// finalizeStuckPendingQuote auto-accepts the latest pending quote when the request
// is still pending and no supplier issue exists yet.
func (s *Service) finalizeStuckPendingQuote(ctx context.Context, tenantID uuid.UUID, pr *PartRequest) {
	if pr == nil || pr.Status != "pending" || pr.Issue != nil {
		return
	}
	var pendingID *uuid.UUID
	var actorID uuid.UUID
	for i := range pr.Quotes {
		if pr.Quotes[i].Status == "pending" {
			id := pr.Quotes[i].ID
			pendingID = &id
			actorID = pr.Quotes[i].SupplierID
			break // quotes are newest-first
		}
	}
	if pendingID == nil {
		return
	}
	var requestedBy uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		SELECT requested_by FROM inventory.part_requests
		WHERE tenant_id = $1 AND id = $2`, tenantID, pr.ID).Scan(&requestedBy); err == nil && requestedBy != uuid.Nil {
		actorID = requestedBy
	}
	result, err := s.AcceptPartRequestQuote(ctx, tenantID, pr.ID, *pendingID, actorID, uuid.New())
	if err != nil {
		return
	}
	pr.Status = "approved"
	accepted := "accepted"
	pr.QuoteStatus = &accepted
	if result != nil && result.Issue != nil {
		pr.Issue = result.Issue
	}
	if quotes, qerr := s.listQuotesForRequest(ctx, tenantID, pr.ID, nil); qerr == nil {
		pr.Quotes = quotes
	}
}

type SupplierIssue struct {
	ID                   uuid.UUID  `json:"id"`
	PartRequestID        uuid.UUID  `json:"part_request_id"`
	RepairJobID          uuid.UUID  `json:"repair_job_id"`
	JobCode              string     `json:"job_code,omitempty"`
	SupplierID           uuid.UUID  `json:"supplier_id"`
	AuthCode             string     `json:"auth_code"`
	Status               string     `json:"status"`
	UnitCost             float64    `json:"unit_cost"`
	CollectedAt          *time.Time `json:"collected_at,omitempty"`
	ReconciliationStatus string     `json:"reconciliation_status"`
}

type Product struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Brand         *string    `json:"brand,omitempty"`
	CategoryID    *uuid.UUID `json:"category_id,omitempty"`
	Category      *string    `json:"category,omitempty"`      // leaf name (denormalized)
	CategoryPath  *string    `json:"category_path,omitempty"` // "Screens › iPhone"
	Description   *string    `json:"description,omitempty"`
	ImageURL      *string    `json:"image_url,omitempty"`
	HasImage      bool       `json:"has_image"`
	ImageUpdated  *time.Time `json:"image_updated_at,omitempty"`
	POSVisible    bool       `json:"pos_visible"`
	OnlineVisible bool       `json:"online_visible"`

	Featured            bool `json:"featured"`
	NewArrival          bool `json:"new_arrival"`
	Bestseller          bool `json:"bestseller"`
	StorefrontSortOrder int  `json:"storefront_sort_order"`
}

type Variant struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
	SKU       string    `json:"sku"`
	SellPrice float64   `json:"sell_price"`
	// What the shop pays for the item. Drives repair job margin when the part is
	// taken off our own shelf, so a zero here shows up as an unpriced part.
	CostPrice float64 `json:"cost_price"`
}

type Reservation struct {
	ID         uuid.UUID `json:"id"`
	VariantID  uuid.UUID `json:"variant_id"`
	LocationID uuid.UUID `json:"location_id"`
	Quantity   int       `json:"quantity"`
	ExpiresAt  time.Time `json:"expires_at"`
	Status     string    `json:"status"`
}

func (s *Service) CreatePartRequest(ctx context.Context, tenantID, branchID, repairJobID uuid.UUID, variantID *uuid.UUID, description string, qty int, supplierID *uuid.UUID, requestedBy, corrID uuid.UUID, clientID *uuid.UUID) (*PartRequest, error) {
	if repairJobID == uuid.Nil {
		return nil, fmt.Errorf("repair_job_id required")
	}
	if qty <= 0 {
		qty = 1
	}
	desc := strings.TrimSpace(description)
	if desc == "" {
		return nil, fmt.Errorf("description required")
	}
	var jobStatus string
	err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairJobID).Scan(&jobStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repair not found")
		}
		return nil, err
	}
	switch jobStatus {
	case "intake", "diagnosed", "waiting_parts", "in_progress":
		// open bench — ok
	default:
		return nil, fmt.Errorf("cannot request parts when the job is %s", jobStatus)
	}
	if supplierID != nil && *supplierID != uuid.Nil {
		var exists bool
		err := s.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM inventory.suppliers WHERE tenant_id = $1 AND id = $2)`,
			tenantID, *supplierID).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("supplier not found")
		}
	}
	if corrID != uuid.Nil {
		var replay PartRequest
		err := s.pool.QueryRow(ctx, `
			SELECT id, repair_job_id, status, description, quantity
			FROM inventory.part_requests
			WHERE tenant_id = $1 AND correlation_id = $2`, tenantID, corrID).Scan(
			&replay.ID, &replay.RepairJobID, &replay.Status, &replay.Description, &replay.Quantity,
		)
		if err == nil {
			return &replay, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	// Block a second open request for the same part on the same job (prevents double-charging).
	var existing uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT id FROM inventory.part_requests
		WHERE tenant_id = $1 AND repair_job_id = $2
		  AND lower(trim(description)) = lower($3)
		  AND status IN ('pending', 'approved')
		LIMIT 1`, tenantID, repairJobID, desc).Scan(&existing)
	if err == nil {
		return nil, fmt.Errorf("an open request for %q already exists on this job", desc)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	id := uuid.New()
	if clientID != nil && *clientID != uuid.Nil {
		id = *clientID
	}
	var assigned *uuid.UUID
	quoteStatus := any(nil)
	if supplierID != nil && *supplierID != uuid.Nil {
		assigned = supplierID
		qs := "awaiting"
		quoteStatus = qs
	}
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.part_requests (
			id, tenant_id, branch_id, repair_job_id, variant_id, description, quantity,
			status, requested_by, correlation_id, assigned_supplier_id, quote_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		id, tenantID, branchID, repairJobID, variantID, desc, qty, requestedBy, corrID, assigned, quoteStatus)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var replay PartRequest
		err = s.pool.QueryRow(ctx, `
			SELECT id, repair_job_id, status, description, quantity
			FROM inventory.part_requests
			WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(
			&replay.ID, &replay.RepairJobID, &replay.Status, &replay.Description, &replay.Quantity,
		)
		if err != nil {
			return nil, err
		}
		return &replay, nil
	}
	// Part requested means the job is blocked on parts until collection.
	s.advanceRepairStatus(ctx, tenantID, repairJobID,
		[]string{"intake", "diagnosed", "in_progress"}, "waiting_parts",
		"Part requested: "+truncateStr(desc, 120), requestedBy, corrID)

	pr := &PartRequest{ID: id, RepairJobID: repairJobID, Status: "pending", Description: desc, Quantity: qty, AssignedSupplierID: assigned}
	if qs, ok := quoteStatus.(string); ok {
		pr.QuoteStatus = &qs
	}
	s.publishPartRequestCreated(ctx, tenantID, branchID, id, repairJobID, desc, qty, assigned, requestedBy, corrID)
	return pr, nil
}

func (s *Service) publishPartRequestCreated(ctx context.Context, tenantID, branchID, requestID, repairJobID uuid.UUID, desc string, qty int, supplierID *uuid.UUID, actorID, corrID uuid.UUID) {
	payload := map[string]any{
		"part_request_id": requestID.String(),
		"repair_job_id":   repairJobID.String(),
		"description":     desc,
		"quantity":        qty,
	}
	var jobCode string
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(job_code, '') FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairJobID).Scan(&jobCode)
	if jobCode != "" {
		payload["job_code"] = jobCode
	}
	if supplierID != nil && *supplierID != uuid.Nil {
		payload["supplier_id"] = supplierID.String()
		var name string
		var phone *string
		_ = s.pool.QueryRow(ctx, `
			SELECT name, phone FROM inventory.suppliers WHERE tenant_id = $1 AND id = $2`,
			tenantID, *supplierID).Scan(&name, &phone)
		if name != "" {
			payload["supplier_name"] = name
		}
		if phone != nil && strings.TrimSpace(*phone) != "" {
			payload["supplier_phone"] = strings.TrimSpace(*phone)
		}
	}
	s.publish("part_request.created", tenantID, branchID, actorID, corrID, payload)
}

// advanceRepairStatus moves a repair job forward when a parts event implies the
// next step (e.g. part requested → waiting_parts, part collected → in_progress).
// Best-effort: a failure here never blocks the parts operation itself.
func (s *Service) advanceRepairStatus(ctx context.Context, tenantID, repairJobID uuid.UUID, from []string, to, note string, actorID, corrID uuid.UUID) {
	if repairJobID == uuid.Nil {
		return
	}
	var current string
	if err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairJobID).Scan(&current); err != nil {
		return
	}
	matched := false
	for _, f := range from {
		if f == current {
			matched = true
			break
		}
	}
	if !matched {
		return
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE repair.repair_jobs SET status = $1, updated_by = $2, updated_at = now(), version = version + 1
		WHERE tenant_id = $3 AND id = $4 AND status = $5`,
		to, actorID, tenantID, repairJobID, current); err != nil {
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairJobID, to, note, actorID, corrID); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func (s *Service) ApprovePartRequest(ctx context.Context, tenantID, requestID uuid.UUID, supplierID *uuid.UUID, unitCost float64, approvedBy, corrID uuid.UUID) (*SupplierIssue, error) {
	var branchID, repairJobID uuid.UUID
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT branch_id, repair_job_id, status FROM inventory.part_requests
		WHERE tenant_id = $1 AND id = $2`, tenantID, requestID).
		Scan(&branchID, &repairJobID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("part request not found")
		}
		return nil, err
	}
	if status != "pending" {
		return nil, fmt.Errorf("part request not pending")
	}

	if supplierID == nil {
		var sid uuid.UUID
		err = s.pool.QueryRow(ctx, `SELECT id FROM inventory.suppliers WHERE tenant_id = $1 ORDER BY created_at LIMIT 1`, tenantID).Scan(&sid)
		if err != nil {
			return nil, fmt.Errorf("no supplier configured")
		}
		supplierID = &sid
	}

	authCode, err := GenerateAuthCode()
	if err != nil {
		return nil, err
	}
	issueID := uuid.New()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE inventory.part_requests SET status = 'approved', updated_at = now() WHERE id = $1`, requestID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_issues (id, tenant_id, branch_id, supplier_id, part_request_id, repair_job_id, auth_code, unit_cost, status, requested_by, approved_by, reconciliation_status, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'approved', $9, $10, 'pending', $11)`,
		issueID, tenantID, branchID, *supplierID, requestID, repairJobID, authCode, unitCost, approvedBy, approvedBy, corrID)
	if err != nil {
		return nil, err
	}
	creditID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_credit_entries (id, tenant_id, supplier_id, supplier_issue_id, amount, entry_type, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, 'issue', $6, $7)`,
		creditID, tenantID, *supplierID, issueID, unitCost, approvedBy, corrID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &SupplierIssue{
		ID: issueID, PartRequestID: requestID, RepairJobID: repairJobID, SupplierID: *supplierID,
		AuthCode: authCode, Status: "approved", UnitCost: unitCost, ReconciliationStatus: "pending",
	}, nil
}

func (s *Service) CollectSupplierIssue(ctx context.Context, tenantID, issueID uuid.UUID, authCode string, collectedBy, corrID uuid.UUID) (*SupplierIssue, error) {
	var storedCode, status string
	var si SupplierIssue
	err := s.pool.QueryRow(ctx, `
		SELECT id, part_request_id, repair_job_id, supplier_id, auth_code, status, unit_cost, reconciliation_status
		FROM inventory.supplier_issues WHERE tenant_id = $1 AND id = $2`, tenantID, issueID).
		Scan(&si.ID, &si.PartRequestID, &si.RepairJobID, &si.SupplierID, &storedCode, &status, &si.UnitCost, &si.ReconciliationStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("supplier issue not found")
		}
		return nil, err
	}
	if status != "approved" {
		return nil, fmt.Errorf("issue not approved")
	}
	if storedCode != authCode {
		return nil, fmt.Errorf("invalid auth_code")
	}
	now := time.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		UPDATE inventory.supplier_issues SET status = 'collected', collected_by = $1, collected_at = $2
		WHERE tenant_id = $3 AND id = $4`, collectedBy, now, tenantID, issueID)
	if err != nil {
		return nil, err
	}
	// Keep the request status honest so ops queues and duplicate guards see fulfilment.
	_, err = tx.Exec(ctx, `
		UPDATE inventory.part_requests SET status = 'collected', updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, si.PartRequestID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	si.Status = "collected"
	si.AuthCode = storedCode
	si.CollectedAt = &now
	var desc string
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(description, 'Part') FROM inventory.part_requests WHERE tenant_id = $1 AND id = $2`,
		tenantID, si.PartRequestID).Scan(&desc)
	s.bookPartCost(ctx, tenantID, si.RepairJobID, "part_supplier", desc, 1, si.UnitCost,
		"supplier_issue", issueID, collectedBy)
	s.onPartFulfilled(ctx, tenantID, si.RepairJobID, "collected from supplier", collectedBy, corrID)
	return &si, nil
}

// ErrInsufficientStock is returned when a part cannot be issued from own stock.
var ErrInsufficientStock = errors.New("insufficient stock at that location")

// IssuePartFromStock fulfils a part request from the shop's own shelf instead of
// sending someone to a supplier. This was the missing branch in the parts flow: a
// part taken off the shop's own stock used to leave no trace at all, so inventory
// drifted and the job looked like pure profit.
func (s *Service) IssuePartFromStock(
	ctx context.Context,
	tenantID, requestID, variantID, locationID uuid.UUID,
	quantity int,
	actorID, corrID uuid.UUID,
) (*PartRequest, error) {
	var branchID, repairJobID uuid.UUID
	var status, description string
	var requestQty int
	err := s.pool.QueryRow(ctx, `
		SELECT branch_id, repair_job_id, status, COALESCE(description, 'Part'), quantity
		FROM inventory.part_requests WHERE tenant_id = $1 AND id = $2`, tenantID, requestID).
		Scan(&branchID, &repairJobID, &status, &description, &requestQty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("part request not found")
		}
		return nil, err
	}
	if status != "pending" && status != "approved" {
		return nil, fmt.Errorf("part request is already %s", status)
	}
	if requestQty <= 0 {
		requestQty = 1
	}
	// Issuing fewer than asked for would close the request while the bench is still
	// short a part, so the whole quantity has to come off the shelf at once.
	if quantity <= 0 {
		quantity = requestQty
	}
	if quantity < requestQty {
		return nil, fmt.Errorf("this request needs %d — issue the full quantity or request the rest separately", requestQty)
	}

	// Cost the part at what the shop paid for it, not what it sells for — this feeds
	// job margin, and marking it up here would flatter every repair.
	var unitCost float64
	var sku, productName string
	err = s.pool.QueryRow(ctx, `
		SELECT v.cost_price::float8, v.sku, p.name
		FROM inventory.product_variants v
		JOIN inventory.products p ON p.id = v.product_id
		WHERE v.tenant_id = $1 AND v.id = $2`, tenantID, variantID).Scan(&unitCost, &sku, &productName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("stock item not found")
		}
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := applyMovementTx(ctx, tx, tenantID, variantID, locationID, -quantity,
		"repair_issue", "part_request", requestID, actorID, corrID); err != nil {
		if strings.Contains(err.Error(), "insufficient stock") {
			return nil, ErrInsufficientStock
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE inventory.part_requests
		SET status = 'issued_from_stock', variant_id = COALESCE(variant_id, $1), updated_at = now()
		WHERE tenant_id = $2 AND id = $3`, variantID, tenantID, requestID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	label := strings.TrimSpace(productName + " (" + sku + ")")
	s.bookPartCost(ctx, tenantID, repairJobID, "part_stock", label, quantity, unitCost,
		"part_request", requestID, actorID)
	s.publish("part_request.issued_from_stock", tenantID, branchID, actorID, corrID, map[string]any{
		"part_request_id": requestID.String(),
		"repair_job_id":   repairJobID.String(),
		"variant_id":      variantID.String(),
		"location_id":     locationID.String(),
		"quantity":        quantity,
		"unit_cost":       unitCost,
	})
	s.onPartFulfilled(ctx, tenantID, repairJobID, "issued from own stock ("+label+")", actorID, corrID)

	return &PartRequest{
		ID: requestID, RepairJobID: repairJobID, Status: "issued_from_stock",
		Description: description, Quantity: requestQty,
	}, nil
}

// onPartFulfilled decides whether a job that was blocked on parts can go back on
// the bench. A job frequently waits on more than one part, so arrival of a single
// part is not enough — moving it to in_progress early makes it look like work is
// happening and hides it from the "waiting parts" queue that chases suppliers.
func (s *Service) onPartFulfilled(ctx context.Context, tenantID, repairJobID uuid.UUID, how string, actorID, corrID uuid.UUID) {
	if repairJobID == uuid.Nil {
		return
	}
	var outstanding int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inventory.part_requests
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status IN ('pending', 'approved')`,
		tenantID, repairJobID).Scan(&outstanding); err != nil {
		return
	}
	if outstanding > 0 {
		// Still blocked. Record the arrival on the timeline anyway so the bench can
		// see progress and nobody re-orders a part that is already in the drawer.
		noun := "parts"
		if outstanding == 1 {
			noun = "part"
		}
		s.noteOnRepair(ctx, tenantID, repairJobID,
			fmt.Sprintf("Part %s — %d more %s still outstanding", how, outstanding, noun),
			actorID, corrID)
		return
	}
	s.advanceRepairStatus(ctx, tenantID, repairJobID,
		[]string{"waiting_parts"}, "in_progress",
		"All parts received ("+how+") — back on the bench", actorID, corrID)
}

// noteOnRepair appends a note to the job timeline without changing its status.
func (s *Service) noteOnRepair(ctx context.Context, tenantID, repairJobID uuid.UUID, note string, actorID, corrID uuid.UUID) {
	var current string
	if err := s.pool.QueryRow(ctx, `
		SELECT status FROM repair.repair_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, repairJobID).Scan(&current); err != nil {
		return
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO repair.repair_status_events (id, tenant_id, repair_job_id, status, note, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.New(), tenantID, repairJobID, current, note, actorID, corrID)
}

// CancelOutstandingPartsForJob voids part requests on a job that has been closed.
// An approved-but-uncollected request is a live authorisation to take a part out
// of a supplier's shelf on the shop's account — leaving those open after the job
// dies is exactly the leak this platform exists to close.
func (s *Service) CancelOutstandingPartsForJob(ctx context.Context, tenantID, repairJobID, actorID uuid.UUID, reason string) (int, error) {
	if repairJobID == uuid.Nil {
		return 0, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(description, '') FROM inventory.part_requests
		WHERE tenant_id = $1 AND repair_job_id = $2 AND status IN ('pending', 'approved')`,
		tenantID, repairJobID)
	if err != nil {
		return 0, err
	}
	type pending struct {
		id   uuid.UUID
		desc string
	}
	var items []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.desc); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	for _, p := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE inventory.part_requests SET status = 'cancelled', updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, p.id); err != nil {
			return 0, err
		}
		// Void the supplier authorisation so the code can no longer be redeemed.
		if _, err := tx.Exec(ctx, `
			UPDATE inventory.supplier_issues SET status = 'cancelled'
			WHERE tenant_id = $1 AND part_request_id = $2 AND status = 'approved'`,
			tenantID, p.id); err != nil {
			return 0, err
		}
		// Reverse the credit booked against the supplier when the issue was approved.
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory.supplier_credit_entries (id, tenant_id, supplier_id, supplier_issue_id, amount, entry_type, created_by, note)
			SELECT $1, si.tenant_id, si.supplier_id, si.id, si.unit_cost, 'adjustment', $2, $3
			FROM inventory.supplier_issues si
			WHERE si.tenant_id = $4 AND si.part_request_id = $5 AND si.status = 'cancelled'`,
			uuid.New(), actorID, "Authorisation voided: "+reason, tenantID, p.id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	s.noteOnRepair(ctx, tenantID, repairJobID,
		fmt.Sprintf("%d outstanding part request(s) voided: %s", len(items), reason), actorID, uuid.Nil)
	return len(items), nil
}

func (s *Service) OrphanIssues(ctx context.Context, tenantID uuid.UUID) ([]SupplierIssue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT si.id, si.part_request_id, si.repair_job_id, COALESCE(rj.job_code, ''), si.supplier_id, si.auth_code, si.status, si.unit_cost, si.collected_at, si.reconciliation_status
		FROM inventory.supplier_issues si
		LEFT JOIN repair.repair_jobs rj ON rj.id = si.repair_job_id
		WHERE si.tenant_id = $1
		  AND si.status = 'collected'
		  AND si.reconciliation_status = 'pending'
		  AND (rj.status IS NULL OR rj.status != 'completed')
		ORDER BY si.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SupplierIssue
	for rows.Next() {
		var si SupplierIssue
		if err := rows.Scan(&si.ID, &si.PartRequestID, &si.RepairJobID, &si.JobCode, &si.SupplierID, &si.AuthCode, &si.Status, &si.UnitCost, &si.CollectedAt, &si.ReconciliationStatus); err != nil {
			return nil, err
		}
		items = append(items, si)
	}
	return items, nil
}

func (s *Service) CreateProduct(
	ctx context.Context,
	tenantID uuid.UUID,
	name string,
	brand *string,
	categoryID *uuid.UUID,
	category, description, imageURL *string,
) (*Product, error) {
	var catName *string
	if categoryID != nil {
		n, err := s.categoryName(ctx, tenantID, *categoryID)
		if err != nil {
			return nil, err
		}
		catName = &n
	} else if category != nil && strings.TrimSpace(*category) != "" {
		// Legacy: free-text creates/finds a root category.
		n := strings.TrimSpace(*category)
		catName = &n
		existing, err := s.findRootCategoryByName(ctx, tenantID, n)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			categoryID = &existing.ID
		} else {
			created, err := s.CreateCategory(ctx, tenantID, n, nil)
			if err != nil {
				return nil, err
			}
			categoryID = &created.ID
		}
	}
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.products (id, tenant_id, name, brand, category_id, category, description, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, tenantID, name, brand, categoryID, catName, description, imageURL)
	if err != nil {
		return nil, err
	}
	return s.getProduct(ctx, tenantID, id)
}

func (s *Service) findRootCategoryByName(ctx context.Context, tenantID uuid.UUID, name string) (*Category, error) {
	var c Category
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, parent_id FROM inventory.categories
		WHERE tenant_id = $1 AND parent_id IS NULL AND lower(name) = lower($2)
		LIMIT 1`, tenantID, strings.TrimSpace(name)).Scan(&c.ID, &c.Name, &c.ParentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) getProduct(ctx context.Context, tenantID, id uuid.UUID) (*Product, error) {
	items, err := s.ListProducts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("product not found")
}

func (s *Service) CreateVariant(ctx context.Context, tenantID, productID uuid.UUID, sku string, sellPrice, costPrice float64) (*Variant, error) {
	id := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price, cost_price)
		VALUES ($1, $2, $3, $4, $5, $6)`, id, tenantID, productID, sku, sellPrice, costPrice)
	if err != nil {
		return nil, err
	}
	return &Variant{ID: id, ProductID: productID, SKU: sku, SellPrice: sellPrice, CostPrice: costPrice}, nil
}

func (s *Service) ListProducts(ctx context.Context, tenantID uuid.UUID) ([]Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.brand, p.category_id, p.category, p.description, p.image_url,
			COALESCE(p.pos_visible, true), COALESCE(p.online_visible, false),
			p.featured, p.new_arrival, p.bestseller, p.storefront_sort_order,
			(p.image_object_key IS NOT NULL OR p.image_bytes IS NOT NULL), p.image_updated_at
		FROM inventory.products p
		WHERE p.tenant_id = $1
		ORDER BY p.name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cats, _ := s.ListCategories(ctx, tenantID)
	pathByID := map[uuid.UUID]string{}
	for _, c := range cats {
		pathByID[c.ID] = c.Path
	}
	var items []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Brand, &p.CategoryID, &p.Category, &p.Description, &p.ImageURL,
			&p.POSVisible, &p.OnlineVisible,
			&p.Featured, &p.NewArrival, &p.Bestseller, &p.StorefrontSortOrder,
			&p.HasImage, &p.ImageUpdated,
		); err != nil {
			return nil, err
		}
		if p.CategoryID != nil {
			if path, ok := pathByID[*p.CategoryID]; ok {
				p.CategoryPath = &path
			}
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Service) UpdateProduct(
	ctx context.Context,
	tenantID, productID uuid.UUID,
	name *string,
	brand *string,
	categoryID **uuid.UUID,
	category, description, imageURL *string,
	posVisible, onlineVisible *bool,
	featured, newArrival, bestseller *bool,
	storefrontSortOrder *int,
) (*Product, error) {
	setCatID := false
	var catIDArg *uuid.UUID
	var catNameArg *string
	if categoryID != nil {
		setCatID = true
		catIDArg = *categoryID
		if catIDArg != nil {
			n, err := s.categoryName(ctx, tenantID, *catIDArg)
			if err != nil {
				return nil, err
			}
			catNameArg = &n
		}
	} else if category != nil {
		// Legacy text field still supported.
		setCatID = false
		catNameArg = category
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE inventory.products SET
			name = COALESCE($1, name),
			brand = CASE WHEN $2::boolean THEN $3 ELSE brand END,
			category_id = CASE WHEN $4::boolean THEN $5 ELSE category_id END,
			category = CASE
				WHEN $4::boolean THEN $6
				WHEN $7::boolean THEN $8
				ELSE category
			END,
			description = CASE WHEN $9::boolean THEN $10 ELSE description END,
			image_url = CASE WHEN $11::boolean THEN $12 ELSE image_url END,
			pos_visible = COALESCE($13, pos_visible),
			online_visible = COALESCE($14, online_visible),
			featured = COALESCE($17, featured),
			new_arrival = COALESCE($18, new_arrival),
			bestseller = COALESCE($19, bestseller),
			storefront_sort_order = COALESCE($20, storefront_sort_order),
			updated_at = now()
		WHERE tenant_id = $15 AND id = $16`,
		name,
		brand != nil, brand,
		setCatID, catIDArg, catNameArg,
		category != nil && !setCatID, category,
		description != nil, description,
		imageURL != nil, imageURL,
		posVisible, onlineVisible,
		tenantID, productID,
		featured, newArrival, bestseller, storefrontSortOrder)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("product not found")
	}
	return s.getProduct(ctx, tenantID, productID)
}

func (s *Service) UpdateVariant(ctx context.Context, tenantID, variantID uuid.UUID, sku *string, sellPrice, costPrice *float64) (*Variant, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE inventory.product_variants SET
			sku = COALESCE($1, sku),
			sell_price = COALESCE($2, sell_price),
			cost_price = COALESCE($3, cost_price)
		WHERE tenant_id = $4 AND id = $5`,
		sku, sellPrice, costPrice, tenantID, variantID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("variant not found")
	}
	var variant Variant
	err = s.pool.QueryRow(ctx, `
		SELECT id, product_id, sku, sell_price::float8, COALESCE(cost_price, 0)::float8
		FROM inventory.product_variants WHERE tenant_id = $1 AND id = $2`,
		tenantID, variantID).Scan(&variant.ID, &variant.ProductID, &variant.SKU, &variant.SellPrice, &variant.CostPrice)
	return &variant, err
}

func (s *Service) ListVariants(ctx context.Context, tenantID uuid.UUID, productID *uuid.UUID) ([]Variant, error) {
	q := `SELECT id, product_id, sku, sell_price, COALESCE(cost_price, 0)::float8
		FROM inventory.product_variants WHERE tenant_id = $1`
	args := []any{tenantID}
	if productID != nil {
		q += ` AND product_id = $2`
		args = append(args, *productID)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Variant
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.SellPrice, &v.CostPrice); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, nil
}

func (s *Service) ReserveInventory(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qty int, ttl time.Duration, refType string, refID uuid.UUID) (*Reservation, error) {
	if qty <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	id := uuid.New()
	expires := time.Now().UTC().Add(ttl)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var available int
	err = tx.QueryRow(ctx, `
		SELECT available_qty FROM inventory.inventory_balances
		WHERE tenant_id = $1 AND variant_id = $2 AND location_id = $3 FOR UPDATE`,
		tenantID, variantID, locationID).Scan(&available)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if available < qty {
		return nil, fmt.Errorf("insufficient available inventory")
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.inventory_balances SET available_qty = available_qty - $1, reserved_qty = reserved_qty + $1, version = version + 1
		WHERE tenant_id = $2 AND variant_id = $3 AND location_id = $4`, qty, tenantID, variantID, locationID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.inventory_reservations (id, tenant_id, variant_id, location_id, quantity, expires_at, reference_type, reference_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, tenantID, variantID, locationID, qty, expires, refType, refID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Reservation{ID: id, VariantID: variantID, LocationID: locationID, Quantity: qty, ExpiresAt: expires, Status: "active"}, nil
}

// ConvertReservation turns an active reservation into a sale movement (reserved → sold).
func (s *Service) ConvertReservation(ctx context.Context, tenantID, reservationID, actorID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var variantID, locationID, refID uuid.UUID
	var qty int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT variant_id, location_id, quantity, status, COALESCE(reference_id, '00000000-0000-0000-0000-000000000000')
		FROM inventory.inventory_reservations
		WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, reservationID).
		Scan(&variantID, &locationID, &qty, &status, &refID)
	if err != nil {
		return err
	}
	if status != "active" {
		return fmt.Errorf("reservation not active")
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.inventory_balances
		SET reserved_qty = reserved_qty - $1, physical_qty = physical_qty - $1, version = version + 1
		WHERE tenant_id=$2 AND variant_id=$3 AND location_id=$4`, qty, tenantID, variantID, locationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.inventory_movements (id, tenant_id, variant_id, location_id, qty_delta, reason, reference_type, reference_id, created_by)
		VALUES ($1,$2,$3,$4,$5,'sale','reservation',$6,$7)`,
		uuid.New(), tenantID, variantID, locationID, -qty, reservationID, actorID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE inventory.inventory_reservations SET status='converted' WHERE id=$1`, reservationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConvertReservationsByReference converts all active reservations for a reference (e.g. online order).
func (s *Service) ConvertReservationsByReference(ctx context.Context, tenantID uuid.UUID, refType string, refID, actorID uuid.UUID) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM inventory.inventory_reservations
		WHERE tenant_id = $1 AND reference_type = $2 AND reference_id = $3 AND status = 'active'`,
		tenantID, refType, refID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return fmt.Errorf("no active reservations for reference")
	}
	for _, id := range ids {
		if err := s.ConvertReservation(ctx, tenantID, id, actorID); err != nil {
			return err
		}
	}
	return nil
}

// ReleaseReservationsByReference releases all active reservations for a reference.
func (s *Service) ReleaseReservationsByReference(ctx context.Context, tenantID uuid.UUID, refType string, refID uuid.UUID) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM inventory.inventory_reservations
		WHERE tenant_id = $1 AND reference_type = $2 AND reference_id = $3 AND status = 'active'`,
		tenantID, refType, refID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := s.ReleaseReservation(ctx, tenantID, id); err != nil {
			return err
		}
	}
	return nil
}

// ReleaseReservation returns reserved qty to available (checkout failure / expiry).
func (s *Service) ReleaseReservation(ctx context.Context, tenantID, reservationID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var variantID, locationID uuid.UUID
	var qty int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT variant_id, location_id, quantity, status FROM inventory.inventory_reservations
		WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, reservationID).
		Scan(&variantID, &locationID, &qty, &status)
	if err != nil {
		return err
	}
	if status != "active" {
		return nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE inventory.inventory_balances
		SET reserved_qty = reserved_qty - $1, available_qty = available_qty + $1, version = version + 1
		WHERE tenant_id=$2 AND variant_id=$3 AND location_id=$4`, qty, tenantID, variantID, locationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE inventory.inventory_reservations SET status='expired' WHERE id=$1`, reservationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ExpireDueReservations releases active reservations past expires_at.
// Returns count released and distinct order reference IDs (reference_type=order).
func (s *Service) ExpireDueReservations(ctx context.Context, tenantID uuid.UUID) (int, []uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(reference_type, ''), COALESCE(reference_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM inventory.inventory_reservations
		WHERE tenant_id = $1 AND status = 'active' AND expires_at <= now()
		ORDER BY expires_at
		LIMIT 200`, tenantID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	type row struct {
		id      uuid.UUID
		refType string
		refID   uuid.UUID
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.refType, &r.refID); err != nil {
			return 0, nil, err
		}
		list = append(list, r)
	}

	orderSet := map[uuid.UUID]struct{}{}
	released := 0
	for _, r := range list {
		if err := s.ReleaseReservation(ctx, tenantID, r.id); err != nil {
			return released, orderIDs(orderSet), err
		}
		released++
		if r.refType == "order" && r.refID != uuid.Nil {
			orderSet[r.refID] = struct{}{}
		}
	}
	return released, orderIDs(orderSet), nil
}

func orderIDs(set map[uuid.UUID]struct{}) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func (s *Service) TransferStock(ctx context.Context, tenantID, variantID, fromLoc, toLoc uuid.UUID, qty int, actorID, corrID uuid.UUID) error {
	if qty <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if fromLoc == toLoc {
		return fmt.Errorf("from and to locations must differ")
	}
	var fromOK, toOK bool
	err := s.pool.QueryRow(ctx, `
		SELECT
			EXISTS(SELECT 1 FROM inventory.stock_locations WHERE id = $1 AND tenant_id = $3),
			EXISTS(SELECT 1 FROM inventory.stock_locations WHERE id = $2 AND tenant_id = $3)`,
		fromLoc, toLoc, tenantID).Scan(&fromOK, &toOK)
	if err != nil {
		return err
	}
	if !fromOK || !toOK {
		return fmt.Errorf("invalid location")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	xferID := uuid.New()
	if err := applyMovementTx(ctx, tx, tenantID, variantID, fromLoc, -qty, "transfer_out", "transfer", xferID, actorID, corrID); err != nil {
		return err
	}
	if err := applyMovementTx(ctx, tx, tenantID, variantID, toLoc, qty, "transfer_in", "transfer", xferID, actorID, corrID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type StockBalance struct {
	VariantID    uuid.UUID `json:"variant_id"`
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	SKU          string    `json:"sku"`
	SellPrice    float64   `json:"sell_price"`
	CostPrice    float64   `json:"cost_price"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`
	PhysicalQty  int       `json:"physical_qty"`
	AvailableQty int       `json:"available_qty"`
	ReservedQty  int       `json:"reserved_qty"`
}

type StockMovement struct {
	ID          uuid.UUID `json:"id"`
	VariantID   uuid.UUID `json:"variant_id"`
	SKU         string    `json:"sku"`
	ProductName string    `json:"product_name"`
	LocationID  uuid.UUID `json:"location_id"`
	QtyDelta    int       `json:"qty_delta"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Service) ListBalances(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID) ([]StockBalance, error) {
	q := `
		SELECT v.id, v.product_id, p.name, v.sku, v.sell_price::float8, COALESCE(v.cost_price, 0)::float8,
		       l.id, l.name, b.physical_qty, b.available_qty, b.reserved_qty
		FROM inventory.inventory_balances b
		JOIN inventory.product_variants v ON v.id = b.variant_id
		JOIN inventory.products p ON p.id = v.product_id
		JOIN inventory.stock_locations l ON l.id = b.location_id
		WHERE b.tenant_id = $1`
	args := []any{tenantID}
	if locationID != nil {
		q += ` AND b.location_id = $2`
		args = append(args, *locationID)
	}
	q += ` ORDER BY p.name, v.sku`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StockBalance
	for rows.Next() {
		var b StockBalance
		if err := rows.Scan(
			&b.VariantID, &b.ProductID, &b.ProductName, &b.SKU, &b.SellPrice, &b.CostPrice,
			&b.LocationID, &b.LocationName, &b.PhysicalQty, &b.AvailableQty, &b.ReservedQty,
		); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	if items == nil {
		items = []StockBalance{}
	}
	return items, nil
}

func (s *Service) ReceiveStock(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qty int, actorID, corrID uuid.UUID, note string) error {
	if qty <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	reason := "receive"
	if strings.TrimSpace(note) != "" {
		reason = "receive:" + truncateStr(note, 40)
	}
	return s.ApplyMovement(ctx, tenantID, variantID, locationID, qty, reason, "manual", corrID, actorID, corrID)
}

func (s *Service) AdjustStock(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qtyDelta int, actorID, corrID uuid.UUID, reason string) error {
	if qtyDelta == 0 {
		return fmt.Errorf("qty_delta must be non-zero")
	}
	if reason == "" {
		reason = "adjustment"
	}
	return s.ApplyMovement(ctx, tenantID, variantID, locationID, qtyDelta, reason, "manual", corrID, actorID, corrID)
}

func (s *Service) ListMovements(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID, limit int) ([]StockMovement, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	q := `
		SELECT m.id, m.variant_id, v.sku, p.name, m.location_id, m.qty_delta, m.reason, m.created_at
		FROM inventory.inventory_movements m
		JOIN inventory.product_variants v ON v.id = m.variant_id
		JOIN inventory.products p ON p.id = v.product_id
		WHERE m.tenant_id = $1`
	args := []any{tenantID}
	n := 2
	if locationID != nil {
		q += fmt.Sprintf(` AND m.location_id = $%d`, n)
		args = append(args, *locationID)
		n++
	}
	q += fmt.Sprintf(` ORDER BY m.created_at DESC LIMIT $%d`, n)
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StockMovement
	for rows.Next() {
		var m StockMovement
		if err := rows.Scan(&m.ID, &m.VariantID, &m.SKU, &m.ProductName, &m.LocationID, &m.QtyDelta, &m.Reason, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	if items == nil {
		items = []StockMovement{}
	}
	return items, nil
}

func truncateStr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *Service) GetVariantPrice(ctx context.Context, tenantID, variantID uuid.UUID) (float64, error) {
	var price float64
	err := s.pool.QueryRow(ctx, `SELECT sell_price FROM inventory.product_variants WHERE tenant_id = $1 AND id = $2`, tenantID, variantID).Scan(&price)
	return price, err
}

func (s *Service) ApplyMovement(ctx context.Context, tenantID, variantID, locationID uuid.UUID, qtyDelta int, reason, refType string, refID, actorID, corrID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := applyMovementTx(ctx, tx, tenantID, variantID, locationID, qtyDelta, reason, refType, refID, actorID, corrID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func applyMovementTx(ctx context.Context, tx pgx.Tx, tenantID, variantID, locationID uuid.UUID, qtyDelta int, reason, refType string, refID, actorID, corrID uuid.UUID) error {
	movID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO inventory.inventory_movements (id, tenant_id, variant_id, location_id, qty_delta, reason, reference_type, reference_id, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		movID, tenantID, variantID, locationID, qtyDelta, reason, refType, refID, actorID, corrID)
	if err != nil {
		return err
	}

	var balID uuid.UUID
	var physical, available int
	err = tx.QueryRow(ctx, `
		SELECT id, physical_qty, available_qty FROM inventory.inventory_balances
		WHERE variant_id = $1 AND location_id = $2 FOR UPDATE`, variantID, locationID).
		Scan(&balID, &physical, &available)
	if errors.Is(err, pgx.ErrNoRows) {
		physical = qtyDelta
		available = qtyDelta
		if available < 0 {
			return fmt.Errorf("insufficient stock")
		}
		balID = uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory.inventory_balances (id, tenant_id, variant_id, location_id, physical_qty, available_qty)
			VALUES ($1, $2, $3, $4, $5, $6)`, balID, tenantID, variantID, locationID, physical, available)
	} else if err == nil {
		physical += qtyDelta
		available += qtyDelta
		if available < 0 {
			return fmt.Errorf("insufficient stock")
		}
		_, err = tx.Exec(ctx, `
			UPDATE inventory.inventory_balances SET physical_qty = $1, available_qty = $2, version = version + 1 WHERE id = $3`,
			physical, available, balID)
	}
	return err
}

type Supplier struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Phone             *string   `json:"phone,omitempty"`
	OutstandingCredit float64   `json:"outstanding_credit"`
	PendingIssueCount int       `json:"pending_issue_count"`
}

type CreditEntry struct {
	ID              uuid.UUID  `json:"id"`
	SupplierID      uuid.UUID  `json:"supplier_id"`
	SupplierIssueID *uuid.UUID `json:"supplier_issue_id,omitempty"`
	Amount          float64    `json:"amount"`
	EntryType       string     `json:"entry_type"`
	CreatedAt       time.Time  `json:"created_at"`
}

type PendingIssue struct {
	ID                   uuid.UUID  `json:"id"`
	PartRequestID        uuid.UUID  `json:"part_request_id"`
	RepairJobID          uuid.UUID  `json:"repair_job_id"`
	JobCode              string     `json:"job_code,omitempty"`
	SupplierID           uuid.UUID  `json:"supplier_id"`
	SupplierName         string     `json:"supplier_name"`
	AuthCode             string     `json:"auth_code"`
	Status               string     `json:"status"`
	UnitCost             float64    `json:"unit_cost"`
	CollectedAt          *time.Time `json:"collected_at,omitempty"`
	ReconciliationStatus string     `json:"reconciliation_status"`
	Description          string     `json:"description"`
}

func (s *Service) ListSuppliers(ctx context.Context, tenantID uuid.UUID) ([]Supplier, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.name, s.phone,
		       COALESCE((
		         SELECT SUM(CASE
		           WHEN e.entry_type = 'issue' THEN e.amount
		           WHEN e.entry_type IN ('settlement', 'payment', 'adjustment') THEN -e.amount
		           ELSE 0 END)
		         FROM inventory.supplier_credit_entries e
		         WHERE e.tenant_id = s.tenant_id AND e.supplier_id = s.id
		       ), 0)::float8,
		       COALESCE((
		         SELECT COUNT(*) FROM inventory.supplier_issues si
		         WHERE si.tenant_id = s.tenant_id AND si.supplier_id = s.id
		           AND si.reconciliation_status = 'pending'
		       ), 0)::int
		FROM inventory.suppliers s
		WHERE s.tenant_id = $1
		ORDER BY s.name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Supplier
	for rows.Next() {
		var sp Supplier
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.Phone, &sp.OutstandingCredit, &sp.PendingIssueCount); err != nil {
			return nil, err
		}
		items = append(items, sp)
	}
	if items == nil {
		items = []Supplier{}
	}
	return items, nil
}

func (s *Service) ListCreditEntries(ctx context.Context, tenantID, supplierID uuid.UUID) ([]CreditEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, supplier_id, supplier_issue_id, amount::float8, entry_type, created_at
		FROM inventory.supplier_credit_entries
		WHERE tenant_id = $1 AND supplier_id = $2
		ORDER BY created_at DESC
		LIMIT 200`, tenantID, supplierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CreditEntry
	for rows.Next() {
		var e CreditEntry
		if err := rows.Scan(&e.ID, &e.SupplierID, &e.SupplierIssueID, &e.Amount, &e.EntryType, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []CreditEntry{}
	}
	return items, nil
}

func (s *Service) ListPendingReconciliation(ctx context.Context, tenantID uuid.UUID) ([]PendingIssue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT si.id, si.part_request_id, si.repair_job_id, COALESCE(rj.job_code, ''),
		       si.supplier_id, s.name, si.auth_code, si.status, si.unit_cost::float8,
		       si.collected_at, si.reconciliation_status, pr.description
		FROM inventory.supplier_issues si
		JOIN inventory.suppliers s ON s.id = si.supplier_id
		JOIN inventory.part_requests pr ON pr.id = si.part_request_id
		LEFT JOIN repair.repair_jobs rj ON rj.id = si.repair_job_id
		WHERE si.tenant_id = $1 AND si.reconciliation_status = 'pending'
		ORDER BY si.created_at DESC
		LIMIT 100`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PendingIssue
	for rows.Next() {
		var p PendingIssue
		if err := rows.Scan(
			&p.ID, &p.PartRequestID, &p.RepairJobID, &p.JobCode,
			&p.SupplierID, &p.SupplierName, &p.AuthCode, &p.Status, &p.UnitCost,
			&p.CollectedAt, &p.ReconciliationStatus, &p.Description,
		); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []PendingIssue{}
	}
	return items, nil
}

func (s *Service) ReconcileSupplierIssue(ctx context.Context, tenantID, issueID, actorID, corrID uuid.UUID) (*SupplierIssue, error) {
	var si SupplierIssue
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT id, part_request_id, repair_job_id, supplier_id, auth_code, status, unit_cost::float8, reconciliation_status
		FROM inventory.supplier_issues WHERE tenant_id = $1 AND id = $2`, tenantID, issueID).
		Scan(&si.ID, &si.PartRequestID, &si.RepairJobID, &si.SupplierID, &si.AuthCode, &status, &si.UnitCost, &si.ReconciliationStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("supplier issue not found")
		}
		return nil, err
	}
	if si.ReconciliationStatus == "reconciled" {
		si.Status = status
		return &si, nil
	}
	// Money must not settle before the part physically changed hands.
	if status != "collected" {
		return nil, fmt.Errorf("issue must be collected before reconcile")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE inventory.supplier_issues
		SET reconciliation_status = 'reconciled'
		WHERE tenant_id = $1 AND id = $2 AND reconciliation_status = 'pending'`, tenantID, issueID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("issue already reconciled")
	}

	settlementID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.supplier_credit_entries (id, tenant_id, supplier_id, supplier_issue_id, amount, entry_type, created_by, correlation_id)
		VALUES ($1, $2, $3, $4, $5, 'settlement', $6, $7)`,
		settlementID, tenantID, si.SupplierID, issueID, si.UnitCost, actorID, corrID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	si.Status = status
	si.ReconciliationStatus = "reconciled"
	if s.alertHook != nil {
		_, _ = s.alertHook.ResolveOpenAlertsByEntity(ctx, tenantID, "orphan_part", issueID, actorID)
	}
	return &si, nil
}

func nullIfBlank(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) productImageKey(ctx context.Context, id uuid.UUID) (string, error) {
	var key *string
	err := s.pool.QueryRow(ctx, `SELECT image_object_key FROM inventory.products WHERE id = $1`, id).Scan(&key)
	if key == nil {
		return "", err
	}
	return *key, err
}

// SaveProductImage stores a product photo for storefront / deals cards.
func (s *Service) SaveProductImage(ctx context.Context, tenantID, id uuid.UUID, body []byte, contentType string) error {
	if len(body) == 0 {
		return errors.New("image file is empty")
	}
	if len(body) > maxImageBytes {
		return fmt.Errorf("image must be %d KB or smaller", maxImageBytes/1024)
	}
	detected, ok := sniffImage(body, contentType)
	if !ok {
		return errors.New("image must be a PNG, JPEG or WebP image")
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM inventory.products WHERE tenant_id = $1 AND id = $2)`, tenantID, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("product not found")
	}

	key := ""
	var dbBytes []byte = body
	if s.store != nil {
		key = fmt.Sprintf("tenants/%s/inventory/products/%s", tenantID, id)
		if err := s.store.Put(ctx, key, body, detected); err != nil {
			key = ""
		} else {
			dbBytes = nil
		}
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE inventory.products
		SET image_object_key = $3, image_bytes = $4, image_content_type = $5, image_updated_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, nullIfBlank(key), dbBytes, detected)
	return err
}

func (s *Service) DeleteProductImage(ctx context.Context, tenantID, id uuid.UUID) error {
	key, err := s.productImageKey(ctx, id)
	if err != nil {
		return err
	}
	if key != "" && s.store != nil {
		_ = s.store.Delete(ctx, key)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE inventory.products
		SET image_object_key = NULL, image_bytes = NULL, image_content_type = NULL, image_updated_at = NULL, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return err
}

// ProductImage returns bytes for the public storefront <img> route.
func (s *Service) ProductImage(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	var key, contentType *string
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT image_object_key, image_content_type, image_bytes
		FROM inventory.products WHERE id = $1`, id).Scan(&key, &contentType, &raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", fmt.Errorf("product not found")
		}
		return nil, "", err
	}
	ct := "image/jpeg"
	if contentType != nil && *contentType != "" {
		ct = *contentType
	}
	if key != nil && *key != "" && s.store != nil {
		body, getErr := s.store.Get(ctx, *key)
		if getErr == nil && len(body) > 0 {
			return body, ct, nil
		}
	}
	if len(raw) == 0 {
		return nil, "", fmt.Errorf("product has no image")
	}
	return raw, ct, nil
}
