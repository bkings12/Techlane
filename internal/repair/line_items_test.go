package repair

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/inventory"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/packages/pkg/db"
)

// ---- Pure-function tests (no DB) ----

func TestApplyJobMoney_LabourLineItemsOverrideLegacyCharge(t *testing.T) {
	j := RepairJob{LaborAmount: 1000, PaidTotal: 0}
	j.LabourTotal = 3000
	j.hasLabourLines = true
	applyJobMoney(&j)
	if j.AmountDue != 3000 {
		t.Fatalf("amount_due=%v want 3000 (line items must win over legacy labor_amount)", j.AmountDue)
	}
}

func TestApplyJobMoney_PartsAddOnTopOfLabour(t *testing.T) {
	j := RepairJob{}
	j.LabourTotal = 3000
	j.hasLabourLines = true
	j.PartsRevenue = 2500
	applyJobMoney(&j)
	if j.AmountDue != 5500 {
		t.Fatalf("amount_due=%v want 5500 (labour + parts)", j.AmountDue)
	}
}

func TestApplyJobMoney_ProductLinesOverrideLegacySaleLines(t *testing.T) {
	j := RepairJob{SaleLinesTotal: 999}
	j.ProductsRevenue = 2800
	j.hasProductLines = true
	applyJobMoney(&j)
	if j.AmountDue != 2800 {
		t.Fatalf("amount_due=%v want 2800 (product lines replace legacy sale_lines total once present)", j.AmountDue)
	}
}

func TestApplyJobMoney_CombinedLabourPartsProducts(t *testing.T) {
	j := RepairJob{PaidTotal: 3000}
	j.LabourTotal = 3000
	j.hasLabourLines = true
	j.PartsRevenue = 2500
	j.ProductsRevenue = 2800
	j.hasProductLines = true
	applyJobMoney(&j)
	if j.AmountDue != 8300 {
		t.Fatalf("amount_due=%v want 8300 (matches the spec's worked example)", j.AmountDue)
	}
	if j.BalanceDue != 5300 {
		t.Fatalf("balance_due=%v want 5300", j.BalanceDue)
	}
}

func TestApplyJobMoney_LegacyJobsUnaffected(t *testing.T) {
	// A job with no line items at all must compute identically to before this
	// feature existed — this is the "additive, non-destructive" guarantee.
	approved := 2500.0
	j := RepairJob{LaborAmount: 1000, ApprovedEstimateTotal: &approved, SaleLinesTotal: 300, PaidTotal: 500}
	applyJobMoney(&j)
	if j.AmountDue != 2800 {
		t.Fatalf("amount_due=%v want 2800 (legacy behavior must be unchanged)", j.AmountDue)
	}
	if j.BalanceDue != 2300 {
		t.Fatalf("balance_due=%v want 2300", j.BalanceDue)
	}
}

func TestApplyJobMoney_AuthorizationUntouchedByLineItems(t *testing.T) {
	auth := 15000.0
	j := RepairJob{AuthorizedAmount: &auth}
	j.ProductsRevenue = 2500
	j.hasProductLines = true
	applyJobMoney(&j)
	if j.AuthorizedAmount == nil || *j.AuthorizedAmount != 15000 {
		t.Fatalf("authorized_amount must never be rewritten by applyJobMoney, got %v", j.AuthorizedAmount)
	}
	if j.AmountDue != 2500 {
		t.Fatalf("amount_due=%v want 2500 (a product line with no labour/part lines contributes on its own)", j.AmountDue)
	}
}

func TestStripLineItemCosts(t *testing.T) {
	cost := 1500.0
	items := []JobLineItem{
		{Description: "Charging IC", UnitCost: &cost, UnitPrice: 2500, PartStatus: strPtr("received")},
	}
	stripped := StripLineItemCosts(items)
	if stripped[0].UnitCost != nil {
		t.Fatalf("unit_cost must be nil after stripping, got %v", *stripped[0].UnitCost)
	}
	if stripped[0].UnitPrice != 2500 {
		t.Fatalf("unit_price must survive stripping, got %v", stripped[0].UnitPrice)
	}
	if stripped[0].PartStatus == nil || *stripped[0].PartStatus != "received" {
		t.Fatalf("part_status must survive stripping")
	}
	// Original slice must not be mutated — a caller with reports.read still
	// needs the real data for a different response.
	if items[0].UnitCost == nil || *items[0].UnitCost != 1500 {
		t.Fatalf("StripLineItemCosts must not mutate its input")
	}
}

func TestFinancialsFromTotals_GrossProfitCorrectness(t *testing.T) {
	// Matches the spec's worked example exactly.
	tot := &LineItemTotals{
		LabourTotal: 3000, HasLabourLines: true,
		PartsRevenue: 2500, PartsCost: 1500, HasPartLines: true,
		ProductsRevenue: 2800, ProductsCost: 1800, HasProductLines: true,
	}
	f := financialsFromTotals(tot)
	if f.PartsProfit != 1000 {
		t.Errorf("parts_profit=%v want 1000", f.PartsProfit)
	}
	if f.ProductsProfit != 1000 {
		t.Errorf("products_profit=%v want 1000", f.ProductsProfit)
	}
	if f.TotalRevenue != 8300 {
		t.Errorf("total_revenue=%v want 8300", f.TotalRevenue)
	}
	if f.TotalCOGS != 3300 {
		t.Errorf("total_cogs=%v want 3300", f.TotalCOGS)
	}
	if f.GrossProfit != 5000 {
		t.Errorf("gross_profit=%v want 5000", f.GrossProfit)
	}
}

func TestFinancialsFromLegacyMargin(t *testing.T) {
	legacy := &JobMargin{LaborAmount: 5000, PartsCost: 2000}
	f := financialsFromLegacyMargin(legacy)
	if !f.UsingLegacyMargin {
		t.Fatal("expected UsingLegacyMargin=true for a job with no line items")
	}
	if f.GrossProfit != 3000 {
		t.Errorf("gross_profit=%v want 3000", f.GrossProfit)
	}
	// A legacy job must report zero product figures, not garbage.
	if f.ProductsRevenue != 0 || f.ProductsCost != 0 {
		t.Errorf("legacy fallback must not fabricate product figures, got revenue=%v cost=%v", f.ProductsRevenue, f.ProductsCost)
	}
}

func strPtr(s string) *string { return &s }

// ---- DB-integration tests (skip if no local Postgres) ----
// Same convention as internal/sales/service_test.go: DATABASE_URL env var,
// localhost default, schemas + migrations applied, skip on connect failure.

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://techlane:techlane@localhost:5433/techlane?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := platform.EnsureSchemas(ctx, pool); err != nil {
		t.Fatalf("schemas: %v", err)
	}
	repoRoot, err := findRepairTestRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if err := platform.RunMigrations(ctx, pool, repoRoot); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func findRepairTestRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// testFixture seeds a minimal tenant/branch/customer/device/repair job plus a
// wired-up inventory service so line items can deduct/restock real stock.
type testFixture struct {
	pool       *pgxpool.Pool
	svc        *Service
	inv        *inventory.Service
	tenantID   uuid.UUID
	branchID   uuid.UUID
	customerID uuid.UUID
	deviceID   uuid.UUID
	repairID   uuid.UUID
	locationID uuid.UUID
	actorID    uuid.UUID
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()
	pool := newTestPool(t)

	invSvc := inventory.NewService(pool)
	svc := NewService(pool, nil)
	svc.SetStockDeductor(inventory.RepairStockAdapter{Svc: invSvc})

	f := &testFixture{
		pool: pool, svc: svc, inv: invSvc,
		tenantID: uuid.New(), branchID: uuid.New(), customerID: uuid.New(),
		deviceID: uuid.New(), repairID: uuid.New(), locationID: uuid.New(), actorID: uuid.New(),
	}

	mustExec(t, pool, `INSERT INTO identity.tenants (id, name) VALUES ($1, 'Test Tenant')`, f.tenantID)
	mustExec(t, pool, `INSERT INTO identity.branches (id, tenant_id, name, code) VALUES ($1, $2, 'Main', 'MAIN')`, f.branchID, f.tenantID)
	mustExec(t, pool, `INSERT INTO repair.customers (id, tenant_id, full_name, phone) VALUES ($1, $2, 'Ayub Macharia', '254700000001')`,
		f.customerID, f.tenantID)
	mustExec(t, pool, `INSERT INTO repair.devices (id, tenant_id, customer_id, kind, brand, model) VALUES ($1, $2, $3, 'laptop', 'Apple', 'MacBook A2337')`,
		f.deviceID, f.tenantID, f.customerID)
	mustExec(t, pool, `INSERT INTO inventory.stock_locations (id, tenant_id, branch_id, name, location_type)
		VALUES ($1, $2, $3, 'Counter', 'shop')`, f.locationID, f.tenantID, f.branchID)

	// Go through the real intake path (job_number/job_code allocation and every
	// other invariant CreateRepair enforces) instead of hand-rolling the insert.
	job, err := svc.CreateRepair(context.Background(), CreateRepairInput{
		TenantID: f.tenantID, BranchID: f.branchID, CustomerID: &f.customerID, DeviceID: f.deviceID,
		ProblemSummary: "No display", ActorID: f.actorID, CorrID: uuid.New(), ClientID: &f.repairID,
	})
	if err != nil {
		t.Fatalf("CreateRepair: %v", err)
	}
	f.repairID = job.ID

	return f
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("seed exec failed: %v\nsql: %s", err, sql)
	}
}

// seedVariant creates a product/variant with stock on hand, returning the variant ID.
func (f *testFixture) seedVariant(t *testing.T, name string, sellPrice, costPrice float64, stockQty int) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	variantID := uuid.New()
	mustExec(t, f.pool, `INSERT INTO inventory.products (id, tenant_id, name) VALUES ($1, $2, $3)`, productID, f.tenantID, name)
	mustExec(t, f.pool, `INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price, cost_price)
		VALUES ($1, $2, $3, $4, $5, $6)`, variantID, f.tenantID, productID, "SKU-"+variantID.String()[:8], sellPrice, costPrice)
	mustExec(t, f.pool, `INSERT INTO inventory.inventory_balances (id, tenant_id, variant_id, location_id, physical_qty, available_qty)
		VALUES ($1, $2, $3, $4, $5, $5)`, uuid.New(), f.tenantID, variantID, f.locationID, stockQty)
	return variantID
}

func (f *testFixture) availableStock(t *testing.T, variantID uuid.UUID) int {
	t.Helper()
	var qty int
	if err := f.pool.QueryRow(context.Background(), `
		SELECT available_qty FROM inventory.inventory_balances WHERE variant_id = $1 AND location_id = $2`,
		variantID, f.locationID).Scan(&qty); err != nil {
		t.Fatalf("read stock: %v", err)
	}
	return qty
}

func TestLineItems_LabourOnly(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	if _, err := f.svc.AddLabourLine(ctx, f.tenantID, f.repairID, "Diagnosis", 500, 1, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddLabourLine: %v", err)
	}
	if _, err := f.svc.AddLabourLine(ctx, f.tenantID, f.repairID, "Board repair", 2500, 1, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddLabourLine: %v", err)
	}

	job, err := f.svc.GetRepair(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("GetRepair: %v", err)
	}
	if job.LabourTotal != 3000 {
		t.Fatalf("labour_total=%v want 3000", job.LabourTotal)
	}
	if job.AmountDue != 3000 {
		t.Fatalf("amount_due=%v want 3000", job.AmountDue)
	}
	if len(job.LabourLines) != 2 {
		t.Fatalf("expected 2 labour lines, got %d", len(job.LabourLines))
	}
	for _, li := range job.LabourLines {
		if li.UnitCost != nil {
			t.Fatalf("labour line must not expose unit_cost on the general job response")
		}
	}
}

func TestLineItems_InventoryPartDeductsStock(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	variantID := f.seedVariant(t, "Charging IC", 2500, 1500, 5)

	line, err := f.svc.AddInventoryPartLine(ctx, f.tenantID, f.repairID, variantID, f.locationID, 1, nil, f.actorID, uuid.New())
	if err != nil {
		t.Fatalf("AddInventoryPartLine: %v", err)
	}
	if line.UnitPrice != 2500 {
		t.Fatalf("unit_price=%v want 2500 (from catalog)", line.UnitPrice)
	}
	if line.UnitCost == nil || *line.UnitCost != 1500 {
		t.Fatalf("unit_cost must be snapshotted from catalog cost_price at insert")
	}
	if line.PartSource == nil || *line.PartSource != PartSourceInventory {
		t.Fatalf("part_source should be 'inventory'")
	}
	if line.PartStatus == nil || *line.PartStatus != PartStatusReceived {
		t.Fatalf("part_status should default to 'received' for stock already on hand")
	}
	if got := f.availableStock(t, variantID); got != 4 {
		t.Fatalf("available stock=%d want 4 after deducting 1", got)
	}
}

func TestLineItems_SourcedPartNoStockTouch(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	supplier := "XYZ Spares"

	line, err := f.svc.AddSourcedPartLine(ctx, f.tenantID, f.repairID,
		"MacBook A2337 Display", &supplier, nil, 14000, 18000, 1, nil, f.actorID, uuid.New())
	if err != nil {
		t.Fatalf("AddSourcedPartLine: %v", err)
	}
	if line.VariantID != nil {
		t.Fatalf("a sourced part must not reference a catalog variant")
	}
	if line.PartSource == nil || *line.PartSource != PartSourceSourced {
		t.Fatalf("part_source should be 'sourced'")
	}
	if line.PartStatus == nil || *line.PartStatus != PartStatusRequired {
		t.Fatalf("part_status should default to 'required'")
	}
	if line.UnitCost == nil || *line.UnitCost != 14000 || line.UnitPrice != 18000 {
		t.Fatalf("cost/price mismatch: cost=%v price=%v", line.UnitCost, line.UnitPrice)
	}

	fin, err := f.svc.JobFinancialsFor(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("JobFinancialsFor: %v", err)
	}
	if fin.PartsRevenue != 18000 || fin.PartsCost != 14000 || fin.PartsProfit != 4000 {
		t.Fatalf("sourced part financials wrong: revenue=%v cost=%v profit=%v", fin.PartsRevenue, fin.PartsCost, fin.PartsProfit)
	}
}

func TestLineItems_MultipleProducts(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	charger := f.seedVariant(t, "Oraimo 45W Charger", 2800, 1800, 10)
	cable := f.seedVariant(t, "USB-C Cable", 800, 400, 10)

	if _, err := f.svc.AddProductLine(ctx, f.tenantID, f.repairID, charger, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddProductLine (charger): %v", err)
	}
	if _, err := f.svc.AddProductLine(ctx, f.tenantID, f.repairID, cable, f.locationID, 2, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddProductLine (cable): %v", err)
	}

	job, err := f.svc.GetRepair(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("GetRepair: %v", err)
	}
	// 2800 + 2*800 = 4400
	if job.ProductsRevenue != 4400 {
		t.Fatalf("products_revenue=%v want 4400", job.ProductsRevenue)
	}
	if len(job.ProductLines) != 2 {
		t.Fatalf("expected 2 product lines, got %d", len(job.ProductLines))
	}
}

func TestLineItems_HistoricalCostPreservation(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	variantID := f.seedVariant(t, "Oraimo 45W Charger", 2800, 1800, 5)

	line, err := f.svc.AddProductLine(ctx, f.tenantID, f.repairID, variantID, f.locationID, 1, nil, f.actorID, uuid.New())
	if err != nil {
		t.Fatalf("AddProductLine: %v", err)
	}
	if *line.UnitCost != 1800 {
		t.Fatalf("initial unit_cost=%v want 1800", *line.UnitCost)
	}

	// Supplier price changes tomorrow — must not rewrite today's sale.
	mustExec(t, f.pool, `UPDATE inventory.product_variants SET cost_price = 2100 WHERE id = $1`, variantID)

	items, err := f.svc.JobLineItems(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("JobLineItems: %v", err)
	}
	if len(items) != 1 || items[0].UnitCost == nil || *items[0].UnitCost != 1800 {
		t.Fatalf("historical unit_cost must stay 1800 despite the catalog cost changing, got %+v", items)
	}
}

func TestLineItems_RemoveReversesStock(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	variantID := f.seedVariant(t, "Screen", 12000, 9000, 3)

	line, err := f.svc.AddInventoryPartLine(ctx, f.tenantID, f.repairID, variantID, f.locationID, 1, nil, f.actorID, uuid.New())
	if err != nil {
		t.Fatalf("AddInventoryPartLine: %v", err)
	}
	if got := f.availableStock(t, variantID); got != 2 {
		t.Fatalf("available stock=%d want 2 after adding", got)
	}

	if err := f.svc.RemoveLineItem(ctx, f.tenantID, f.repairID, line.ID, f.actorID, uuid.New()); err != nil {
		t.Fatalf("RemoveLineItem: %v", err)
	}
	if got := f.availableStock(t, variantID); got != 3 {
		t.Fatalf("available stock=%d want 3 after removal (never silently lose stock)", got)
	}

	items, err := f.svc.JobLineItems(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("JobLineItems: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected the line item to be gone after removal, got %d", len(items))
	}
}

func TestLineItems_CombinedTotalPartialAndFullPayment(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	part := f.seedVariant(t, "Charging IC", 2500, 1500, 5)
	product := f.seedVariant(t, "Oraimo 45W Charger", 2800, 1800, 5)

	if _, err := f.svc.AddLabourLine(ctx, f.tenantID, f.repairID, "Logic board repair", 3000, 1, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddLabourLine: %v", err)
	}
	if _, err := f.svc.AddInventoryPartLine(ctx, f.tenantID, f.repairID, part, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddInventoryPartLine: %v", err)
	}
	if _, err := f.svc.AddProductLine(ctx, f.tenantID, f.repairID, product, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddProductLine: %v", err)
	}

	job, err := f.svc.GetRepair(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("GetRepair: %v", err)
	}
	if job.AmountDue != 8300 {
		t.Fatalf("amount_due=%v want 8300 (matches the spec's worked example)", job.AmountDue)
	}

	// Partial payment.
	seedPayment(t, f.pool, f.tenantID, f.repairID, 3000, f.actorID)
	job, err = f.svc.GetRepair(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("GetRepair: %v", err)
	}
	if job.PaidTotal != 3000 {
		t.Fatalf("paid_total=%v want 3000", job.PaidTotal)
	}
	if job.BalanceDue != 5300 {
		t.Fatalf("balance_due=%v want 5300 after partial payment", job.BalanceDue)
	}

	// Full payment.
	seedPayment(t, f.pool, f.tenantID, f.repairID, 5300, f.actorID)
	job, err = f.svc.GetRepair(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("GetRepair: %v", err)
	}
	if job.BalanceDue != 0 {
		t.Fatalf("balance_due=%v want 0 after full payment", job.BalanceDue)
	}
}

// seedPayment inserts a confirmed payment allocated to a repair, matching the
// shape internal/payments would produce — kept self-contained here rather
// than importing internal/payments, since this test only needs to verify
// internal/repair's own balance computation, not the payments service.
func seedPayment(t *testing.T, pool *pgxpool.Pool, tenantID, repairID uuid.UUID, amount float64, actorID uuid.UUID) {
	t.Helper()
	paymentID := uuid.New()
	mustExec(t, pool, `INSERT INTO payments.payments (id, tenant_id, method, amount, currency, status, received_by, created_by)
		VALUES ($1, $2, 'cash', $3, 'KES', 'confirmed', $4, $4)`, paymentID, tenantID, amount, actorID)
	mustExec(t, pool, `INSERT INTO payments.payment_allocations (id, tenant_id, payment_id, payable_type, payable_id, amount)
		VALUES ($1, $2, $3, 'repair', $4, $5)`, uuid.New(), tenantID, paymentID, repairID, amount)
}

func TestLineItems_AuthorizationUnaffectedByLaterProductAddition(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	// Simulate an approved estimate: labour + required parts authorized at 15000.
	now := time.Now().UTC()
	mustExec(t, f.pool, `UPDATE repair.repair_jobs
		SET labor_amount = 15000, authorized_amount = 15000, work_authorized_at = $1, work_authorization_source = 'customer_estimate'
		WHERE id = $2`, now, f.repairID)

	product := f.seedVariant(t, "Charger", 2500, 1500, 5)
	if _, err := f.svc.AddProductLine(ctx, f.tenantID, f.repairID, product, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddProductLine: %v", err)
	}

	job, err := f.svc.GetRepair(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("GetRepair: %v", err)
	}
	if job.AuthorizedAmount == nil || *job.AuthorizedAmount != 15000 {
		t.Fatalf("authorized_amount must stay 15000 after a later product addition, got %v", job.AuthorizedAmount)
	}
	if job.AmountDue != 17500 {
		t.Fatalf("amount_due=%v want 17500 (15000 authorized repair + 2500 product)", job.AmountDue)
	}
}

func TestCustomerLifetimeStats_NoDoubleCounting(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	if _, err := f.svc.AddLabourLine(ctx, f.tenantID, f.repairID, "Board repair", 5000, 1, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddLabourLine: %v", err)
	}
	part := f.seedVariant(t, "Battery", 3000, 2000, 5)
	if _, err := f.svc.AddInventoryPartLine(ctx, f.tenantID, f.repairID, part, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddInventoryPartLine: %v", err)
	}
	product := f.seedVariant(t, "Case", 1000, 600, 5)
	if _, err := f.svc.AddProductLine(ctx, f.tenantID, f.repairID, product, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddProductLine: %v", err)
	}

	// A standalone POS sale for the same customer, unrelated to the repair.
	saleID := uuid.New()
	mustExec(t, f.pool, `INSERT INTO sales.sales (id, tenant_id, branch_id, customer_id, channel, status, subtotal, total, created_by)
		VALUES ($1, $2, $3, $4, 'pos', 'completed', 500, 500, $5)`, saleID, f.tenantID, f.branchID, f.customerID, f.actorID)
	mustExec(t, f.pool, `INSERT INTO sales.sale_items (id, tenant_id, sale_id, quantity, unit_price, line_total, description)
		VALUES ($1, $2, $3, 1, 500, 500, 'Screen protector')`, uuid.New(), f.tenantID, saleID)

	stats, err := f.svc.CustomerLifetimeStats(ctx, f.tenantID, f.customerID)
	if err != nil {
		t.Fatalf("CustomerLifetimeStats: %v", err)
	}
	if stats.RepairsCount != 1 {
		t.Fatalf("repairs_count=%d want 1", stats.RepairsCount)
	}
	if stats.RepairsRevenue != 5000 {
		t.Fatalf("repairs_revenue=%v want 5000", stats.RepairsRevenue)
	}
	if stats.RepairPartsRevenue != 3000 {
		t.Fatalf("repair_parts_revenue=%v want 3000", stats.RepairPartsRevenue)
	}
	// 1000 (repair-attached product) + 500 (standalone sale) = 1500, not double-counted.
	if stats.AccessoriesRevenue != 1500 {
		t.Fatalf("accessories_revenue=%v want 1500 (repair product + standalone sale, no double count)", stats.AccessoriesRevenue)
	}
	if stats.LifetimeSpend != 9500 {
		t.Fatalf("lifetime_spend=%v want 9500", stats.LifetimeSpend)
	}
}

func TestBuildCustomerReceipt_ExcludesCost(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	if _, err := f.svc.AddLabourLine(ctx, f.tenantID, f.repairID, "Board repair", 3000, 1, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddLabourLine: %v", err)
	}
	part := f.seedVariant(t, "Charging IC", 2500, 1500, 5)
	if _, err := f.svc.AddInventoryPartLine(ctx, f.tenantID, f.repairID, part, f.locationID, 1, nil, f.actorID, uuid.New()); err != nil {
		t.Fatalf("AddInventoryPartLine: %v", err)
	}

	doc, err := f.svc.BuildCustomerReceipt(ctx, f.tenantID, f.repairID)
	if err != nil {
		t.Fatalf("BuildCustomerReceipt: %v", err)
	}
	if len(doc.PartLines) != 1 {
		t.Fatalf("expected 1 part line on the receipt, got %d", len(doc.PartLines))
	}
	if doc.PartLines[0].UnitCost != nil {
		t.Fatalf("receipt document must never carry a cost basis, got %v", *doc.PartLines[0].UnitCost)
	}

	// The shared cross-module receipt model has no cost field at all to leak into.
	rdoc := doc.ToReceiptDocument(false)
	for _, line := range rdoc.Lines {
		_ = line // receipts.Line has no cost/margin field — compile-time guarantee, not a runtime check.
	}
}
