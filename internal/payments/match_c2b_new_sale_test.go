package payments_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/inventory"
	"github.com/techlane/techlane/internal/payments"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/internal/sales"
	"github.com/techlane/techlane/packages/pkg/db"
)

// TestMatchC2BToNewSale is a DB-integration test — needs a real Postgres (skips
// otherwise, same convention as internal/sync and internal/sales).
func TestMatchC2BToNewSale(t *testing.T) {
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

	tenantID, branchID, locationID, actorID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	productID, variantID := uuid.New(), uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO inventory.products (id, tenant_id, name) VALUES ($1, $2, 'Test Product')`, productID, tenantID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price, cost_price)
		VALUES ($1, $2, $3, 'TEST-SKU-1', 1500, 900)`, variantID, tenantID, productID); err != nil {
		t.Fatalf("seed variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO inventory.stock_locations (id, tenant_id, branch_id, name, location_type)
		VALUES ($1, $2, $3, 'Test Counter', 'counter')`, locationID, tenantID, branchID); err != nil {
		t.Fatalf("seed location: %v", err)
	}

	invSvc := inventory.NewService(pool)
	if err := invSvc.ApplyMovement(ctx, tenantID, variantID, locationID, 5, "test_seed", "test", uuid.New(), actorID, uuid.New()); err != nil {
		t.Fatalf("seed stock: %v", err)
	}

	salesSvc := sales.NewService(pool, invSvc)
	paySvc := payments.NewService(pool)
	paySvc.SetQuickSaleCreator(payments.SalesQuickSaleAdapter{Svc: salesSvc})

	c2bID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payments.mpesa_c2b_transactions (id, tenant_id, payment_id, trans_id, amount, status)
		VALUES ($1, $2, NULL, 'TEST-TRANS-1', 1500, 'unmatched')`, c2bID, tenantID); err != nil {
		t.Fatalf("seed c2b transaction: %v", err)
	}

	pay, err := paySvc.MatchC2BToNewSale(ctx, tenantID, c2bID, payments.MatchC2BToNewSaleInput{
		BranchID: branchID, LocationID: locationID, VariantID: variantID, Quantity: 1,
	}, actorID)
	if err != nil {
		t.Fatalf("MatchC2BToNewSale: %v", err)
	}
	if pay.Status != "confirmed" && pay.Status != "allocated" {
		t.Errorf("expected payment status confirmed/allocated, got %q", pay.Status)
	}
	if pay.Amount != 1500 {
		t.Errorf("expected payment amount 1500, got %v", pay.Amount)
	}

	var available int
	if err := pool.QueryRow(ctx, `
		SELECT available_qty FROM inventory.inventory_balances WHERE variant_id = $1 AND location_id = $2`,
		variantID, locationID).Scan(&available); err != nil {
		t.Fatalf("query balance: %v", err)
	}
	if available != 4 {
		t.Errorf("expected stock to drop from 5 to 4, got %d", available)
	}

	var c2bStatus string
	var paymentID *uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT status, payment_id FROM payments.mpesa_c2b_transactions WHERE id = $1`, c2bID).
		Scan(&c2bStatus, &paymentID); err != nil {
		t.Fatalf("query c2b: %v", err)
	}
	if c2bStatus == "unmatched" {
		t.Errorf("expected c2b transaction to no longer be unmatched, got %q", c2bStatus)
	}
	if paymentID == nil || *paymentID != pay.ID {
		t.Errorf("expected c2b transaction payment_id to be %s, got %v", pay.ID, paymentID)
	}

	var saleStatus string
	var saleTotal float64
	if err := pool.QueryRow(ctx, `
		SELECT s.status, s.total::float8
		FROM payments.payment_allocations a
		JOIN sales.sales s ON s.id = a.payable_id AND a.payable_type = 'sale'
		WHERE a.payment_id = $1`, pay.ID).Scan(&saleStatus, &saleTotal); err != nil {
		t.Fatalf("query sale: %v", err)
	}
	if saleStatus != "completed" {
		t.Errorf("expected sale status completed, got %q", saleStatus)
	}
	if saleTotal != 1500 {
		t.Errorf("expected sale total 1500, got %v", saleTotal)
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
