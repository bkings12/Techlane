package sales_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
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
