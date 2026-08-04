package sales_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/internal/sales"
	"github.com/techlane/techlane/packages/pkg/db"
)

func ptr(f float64) *float64 { return &f }

// Quick-sale items skip the catalog price lookup, so validation runs before any
// pool access — these cases need no database at all.
func TestCreateSale_QuickSaleValidation(t *testing.T) {
	svc := sales.NewService(nil, nil)
	tenantID, branchID, actorID := uuid.New(), uuid.New(), uuid.New()
	supplierID := uuid.New()

	cases := []struct {
		name    string
		items   []sales.SaleItemInput
		wantErr string
	}{
		{
			name:    "missing description",
			items:   []sales.SaleItemInput{{Quantity: 1, UnitPrice: ptr(500)}},
			wantErr: "description required",
		},
		{
			name:    "missing unit price",
			items:   []sales.SaleItemInput{{Quantity: 1, Description: "Screen protector"}},
			wantErr: "unit_price required",
		},
		{
			name:    "zero unit price",
			items:   []sales.SaleItemInput{{Quantity: 1, Description: "Screen protector", UnitPrice: ptr(0)}},
			wantErr: "unit_price required",
		},
		{
			name: "supplier without cost",
			items: []sales.SaleItemInput{{
				Quantity: 1, Description: "Screen protector", UnitPrice: ptr(500), SupplierID: &supplierID,
			}},
			wantErr: "must be provided together",
		},
		{
			name: "cost without supplier",
			items: []sales.SaleItemInput{{
				Quantity: 1, Description: "Screen protector", UnitPrice: ptr(500), UnitCost: ptr(300),
			}},
			wantErr: "must be provided together",
		},
		{
			name: "negative cost",
			items: []sales.SaleItemInput{{
				Quantity: 1, Description: "Screen protector", UnitPrice: ptr(500), UnitCost: ptr(-1), SupplierID: &supplierID,
			}},
			wantErr: "cannot be negative",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateSale(context.Background(), sales.CreateSaleInput{
				TenantID: tenantID, BranchID: branchID, Items: tc.items, ActorID: actorID, CorrID: uuid.New(),
			})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestCreateSale_QuickSaleRecordsSupplierCredit is a DB-integration test — it needs a
// real Postgres (skips otherwise, same convention as internal/sync's tests).
func TestCreateSale_QuickSaleRecordsSupplierCredit(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	defer pool.Close()
	if err := platform.EnsureSchemas(ctx, pool); err != nil {
		t.Fatalf("schemas: %v", err)
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if err := platform.RunMigrations(ctx, pool, repoRoot); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	tenantID, branchID, actorID := uuid.New(), uuid.New(), uuid.New()
	supplierID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO inventory.suppliers (id, tenant_id, name) VALUES ($1, $2, 'Test Supplier')`,
		supplierID, tenantID); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}

	svc := sales.NewService(pool, nil)
	sale, err := svc.CreateSale(ctx, sales.CreateSaleInput{
		TenantID: tenantID, BranchID: branchID, ActorID: actorID, CorrID: uuid.New(),
		Items: []sales.SaleItemInput{{
			Quantity: 2, Description: "Outsourced screen", UnitPrice: ptr(2500), UnitCost: ptr(1800), SupplierID: &supplierID,
		}},
	})
	if err != nil {
		t.Fatalf("CreateSale: %v", err)
	}
	if len(sale.Items) != 1 {
		t.Fatalf("expected 1 line item, got %d", len(sale.Items))
	}
	line := sale.Items[0]
	if line.VariantID != uuid.Nil {
		t.Errorf("expected nil variant_id for quick-sale line, got %s", line.VariantID)
	}
	if line.LineTotal != 5000 {
		t.Errorf("expected line_total 5000, got %v", line.LineTotal)
	}
	if line.Margin == nil || *line.Margin != 1400 {
		t.Errorf("expected margin 1400 (5000 - 1800*2), got %v", line.Margin)
	}
	if sale.Subtotal != 5000 {
		t.Errorf("expected subtotal 5000, got %v", sale.Subtotal)
	}

	var count int
	var amount float64
	var entryType, note string
	err = pool.QueryRow(ctx, `
		SELECT count(*), sum(amount)::float8, max(entry_type), max(note)
		FROM inventory.supplier_credit_entries WHERE tenant_id = $1 AND supplier_id = $2`,
		tenantID, supplierID).Scan(&count, &amount, &entryType, &note)
	if err != nil {
		t.Fatalf("query credit entries: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 supplier credit entry, got %d", count)
	}
	if amount != 3600 {
		t.Errorf("expected credit amount 3600 (1800*2), got %v", amount)
	}
	if entryType != "issue" {
		t.Errorf("expected entry_type 'issue', got %q", entryType)
	}
	if !contains(note, "Quick sale") {
		t.Errorf("expected note to mention Quick sale, got %q", note)
	}
}

func newTestPool(t *testing.T) (*pgxpool.Pool, error) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := platform.EnsureSchemas(ctx, pool); err != nil {
		t.Fatalf("schemas: %v", err)
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if err := platform.RunMigrations(ctx, pool, repoRoot); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return pool, nil
}

// TestGetSale_RichDetailFields covers the fields Sale Details needs that the
// old thin Sale struct never populated: short reference, branch/cashier name,
// resolved M-Pesa payment reference, paid/balance.
func TestGetSale_RichDetailFields(t *testing.T) {
	pool, err := newTestPool(t)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	defer pool.Close()
	ctx := context.Background()

	tenantID, branchID, actorID := uuid.New(), uuid.New(), uuid.New()
	mustExecSales(t, pool, `INSERT INTO identity.tenants (id, name) VALUES ($1, 'Test Tenant')`, tenantID)
	mustExecSales(t, pool, `INSERT INTO identity.branches (id, tenant_id, name, code) VALUES ($1, $2, 'Main', 'MAIN')`, branchID, tenantID)
	mustExecSales(t, pool, `INSERT INTO identity.users (id, tenant_id, email, display_name, password_hash)
		VALUES ($1, $2, 'cashier@test.local', 'Jane Cashier', 'x')`, actorID, tenantID)

	svc := sales.NewService(pool, nil)
	sale, err := svc.CreateSale(ctx, sales.CreateSaleInput{
		TenantID: tenantID, BranchID: branchID, ActorID: actorID, CorrID: uuid.New(),
		Items: []sales.SaleItemInput{{Quantity: 1, Description: "Screen protector", UnitPrice: ptr(500)}},
	})
	if err != nil {
		t.Fatalf("CreateSale: %v", err)
	}
	// CreateSale doesn't stamp created_by (that's the handler's claims.UserID
	// path in production); set it directly so the cashier-name resolution has
	// something to join against.
	mustExecSales(t, pool, `UPDATE sales.sales SET created_by = $1 WHERE id = $2`, actorID, sale.ID)

	paymentID := uuid.New()
	mustExecSales(t, pool, `INSERT INTO payments.payments (id, tenant_id, method, amount, currency, status, created_by)
		VALUES ($1, $2, 'mpesa_stk', 500, 'KES', 'confirmed', $3)`, paymentID, tenantID, actorID)
	mustExecSales(t, pool, `INSERT INTO payments.payment_allocations (id, tenant_id, payment_id, payable_type, payable_id, amount)
		VALUES ($1, $2, $3, 'sale', $4, 500)`, uuid.New(), tenantID, paymentID, sale.ID)
	mustExecSales(t, pool, `INSERT INTO payments.mpesa_stk_transactions (id, tenant_id, payment_id, mpesa_receipt, phone, status)
		VALUES ($1, $2, $3, 'QHK7T9X001', '254700000001', 'confirmed')`, uuid.New(), tenantID, paymentID)

	got, err := svc.GetSale(ctx, tenantID, sale.ID)
	if err != nil {
		t.Fatalf("GetSale: %v", err)
	}
	if got.Reference == "" || !contains(got.Reference, "SL-") {
		t.Errorf("expected a short reference like SL-XXXXXXXX, got %q", got.Reference)
	}
	if got.BranchName != "Main" {
		t.Errorf("branch_name=%q want Main", got.BranchName)
	}
	if got.CashierName != "Jane Cashier" {
		t.Errorf("cashier_name=%q want Jane Cashier", got.CashierName)
	}
	if got.PaymentReference != "QHK7T9X001" {
		t.Errorf("payment_reference=%q want QHK7T9X001", got.PaymentReference)
	}
	if got.PaymentStatus != "confirmed" {
		t.Errorf("payment_status=%q want confirmed", got.PaymentStatus)
	}
	if got.PaidTotal != 500 {
		t.Errorf("paid_total=%v want 500", got.PaidTotal)
	}
	if got.BalanceDue != 0 {
		t.Errorf("balance_due=%v want 0", got.BalanceDue)
	}
}

func TestListSales_SearchAndFilters(t *testing.T) {
	pool, err := newTestPool(t)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	defer pool.Close()
	ctx := context.Background()

	tenantID, branchID, actorID := uuid.New(), uuid.New(), uuid.New()
	mustExecSales(t, pool, `INSERT INTO identity.tenants (id, name) VALUES ($1, 'Test Tenant')`, tenantID)
	mustExecSales(t, pool, `INSERT INTO identity.branches (id, tenant_id, name, code) VALUES ($1, $2, 'Main', 'MAIN')`, branchID, tenantID)
	customerID := uuid.New()
	mustExecSales(t, pool, `INSERT INTO repair.customers (id, tenant_id, full_name, phone) VALUES ($1, $2, 'Ayub Macharia', '254700000099')`,
		customerID, tenantID)

	svc := sales.NewService(pool, nil)
	sale, err := svc.CreateSale(ctx, sales.CreateSaleInput{
		TenantID: tenantID, BranchID: branchID, CustomerID: &customerID, ActorID: actorID, CorrID: uuid.New(),
		Items: []sales.SaleItemInput{{Quantity: 1, Description: "Oraimo 45W Charger", UnitPrice: ptr(2800)}},
	})
	if err != nil {
		t.Fatalf("CreateSale: %v", err)
	}
	paymentID := uuid.New()
	mustExecSales(t, pool, `INSERT INTO payments.payments (id, tenant_id, method, amount, currency, status, created_by)
		VALUES ($1, $2, 'mpesa_stk', 2800, 'KES', 'confirmed', $3)`, paymentID, tenantID, actorID)
	mustExecSales(t, pool, `INSERT INTO payments.payment_allocations (id, tenant_id, payment_id, payable_type, payable_id, amount)
		VALUES ($1, $2, $3, 'sale', $4, 2800)`, uuid.New(), tenantID, paymentID, sale.ID)
	mustExecSales(t, pool, `INSERT INTO payments.mpesa_stk_transactions (id, tenant_id, payment_id, mpesa_receipt, phone, status)
		VALUES ($1, $2, $3, 'QHK9ZZZ999', '254700000099', 'confirmed')`, uuid.New(), tenantID, paymentID)

	cases := []struct {
		name   string
		filter sales.ListSalesFilter
		want   bool
	}{
		{"by customer name", sales.ListSalesFilter{Query: "Macharia"}, true},
		{"by phone", sales.ListSalesFilter{Query: "254700000099"}, true},
		{"by mpesa reference", sales.ListSalesFilter{Query: "QHK9ZZZ999"}, true},
		{"by product name", sales.ListSalesFilter{Query: "Oraimo"}, true},
		{"by unrelated text", sales.ListSalesFilter{Query: "nonexistent-xyz"}, false},
		{"by method match", sales.ListSalesFilter{Method: "mpesa_stk"}, true},
		{"by method mismatch", sales.ListSalesFilter{Method: "cash"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := svc.ListSales(ctx, tenantID, tc.filter)
			if err != nil {
				t.Fatalf("ListSales: %v", err)
			}
			found := false
			for _, it := range items {
				if it.ID == sale.ID {
					found = true
				}
			}
			if found != tc.want {
				t.Errorf("found=%v want=%v (filter=%+v)", found, tc.want, tc.filter)
			}
		})
	}

	// Date range: a "from" in the future must exclude today's sale.
	future := time.Now().UTC().Add(24 * time.Hour)
	items, err := svc.ListSales(ctx, tenantID, sales.ListSalesFilter{From: &future})
	if err != nil {
		t.Fatalf("ListSales: %v", err)
	}
	for _, it := range items {
		if it.ID == sale.ID {
			t.Errorf("a from-tomorrow filter must not include a sale created today")
		}
	}
}

func mustExecSales(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("seed exec failed: %v\nsql: %s", err, sql)
	}
}

func findRepoRoot() (string, error) {
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
