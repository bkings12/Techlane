package commerce_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/commerce"
	"github.com/techlane/techlane/internal/inventory"
	"github.com/techlane/techlane/internal/platform"
	"github.com/techlane/techlane/internal/storefrontcms"
	"github.com/techlane/techlane/packages/pkg/db"
)

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

// TestStartCheckout_DealPricing is a DB-integration test — needs a real
// Postgres (skips otherwise, same convention as internal/sync and
// internal/sales). It proves storefront deals apply a real discount at
// checkout, not a decorative one: the customer is actually charged the deal
// price, and an expired deal charges the full base price.
func TestStartCheckout_DealPricing(t *testing.T) {
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

	tenantID, branchID, locationID := uuid.New(), uuid.New(), uuid.New()
	productID, dealVariantID, plainVariantID := uuid.New(), uuid.New(), uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO inventory.products (id, tenant_id, name, online_visible) VALUES ($1, $2, 'Test Product', true)`, productID, tenantID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price, cost_price)
		VALUES ($1, $2, $3, 'DEAL-SKU', 1000, 600)`, dealVariantID, tenantID, productID); err != nil {
		t.Fatalf("seed deal variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price, cost_price)
		VALUES ($1, $2, $3, 'EXPIRED-SKU', 1000, 600)`, plainVariantID, tenantID, productID); err != nil {
		t.Fatalf("seed expired-deal variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO inventory.stock_locations (id, tenant_id, branch_id, name, location_type)
		VALUES ($1, $2, $3, 'Test Front', 'front')`, locationID, tenantID, branchID); err != nil {
		t.Fatalf("seed location: %v", err)
	}

	invSvc := inventory.NewService(pool)
	actorID := uuid.New()
	if err := invSvc.ApplyMovement(ctx, tenantID, dealVariantID, locationID, 10, "test_seed", "test", uuid.New(), actorID, uuid.New()); err != nil {
		t.Fatalf("seed deal stock: %v", err)
	}
	if err := invSvc.ApplyMovement(ctx, tenantID, plainVariantID, locationID, 10, "test_seed", "test", uuid.New(), actorID, uuid.New()); err != nil {
		t.Fatalf("seed expired-deal stock: %v", err)
	}

	sfSvc := storefrontcms.NewService(pool)
	if _, err := sfSvc.CreateDeal(ctx, tenantID, storefrontcms.DealInput{
		VariantID: &dealVariantID, DealPrice: floatPtr(750), Active: boolPtr(true),
	}); err != nil {
		t.Fatalf("create active deal: %v", err)
	}
	past := time.Now().UTC().Add(-1 * time.Hour)
	pastPtr := &past
	if _, err := sfSvc.CreateDeal(ctx, tenantID, storefrontcms.DealInput{
		VariantID: &plainVariantID, DealPrice: floatPtr(500), Active: boolPtr(true), EndsAt: &pastPtr,
	}); err != nil {
		t.Fatalf("create expired deal: %v", err)
	}

	commerceSvc := commerce.NewService(pool, invSvc, sfSvc)

	// Active deal: must charge the deal price, not the base price.
	result, err := commerceSvc.StartCheckout(ctx, tenantID, commerce.CheckoutRequest{
		BranchID: branchID, LocationID: locationID,
		CustomerName: "Test Buyer", Phone: "0712345678",
		Items: []commerce.CartItemInput{{VariantID: dealVariantID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("StartCheckout (active deal): %v", err)
	}
	if result.Order.Total != 750 {
		t.Errorf("expected active-deal order total 750, got %v", result.Order.Total)
	}
	var unitPrice, originalPrice float64
	if err := pool.QueryRow(ctx, `
		SELECT unit_price::float8, original_unit_price::float8 FROM sales.order_items WHERE order_id = $1`, result.Order.ID).
		Scan(&unitPrice, &originalPrice); err != nil {
		t.Fatalf("query order_items: %v", err)
	}
	if unitPrice != 750 {
		t.Errorf("expected order_items.unit_price 750, got %v", unitPrice)
	}
	if originalPrice != 1000 {
		t.Errorf("expected order_items.original_unit_price 1000, got %v", originalPrice)
	}

	// Expired deal: must charge the full base price.
	result2, err := commerceSvc.StartCheckout(ctx, tenantID, commerce.CheckoutRequest{
		BranchID: branchID, LocationID: locationID,
		CustomerName: "Test Buyer", Phone: "0712345678",
		Items: []commerce.CartItemInput{{VariantID: plainVariantID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("StartCheckout (expired deal): %v", err)
	}
	if result2.Order.Total != 1000 {
		t.Errorf("expected expired-deal order total 1000 (full price), got %v", result2.Order.Total)
	}
}

func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool        { return &b }
