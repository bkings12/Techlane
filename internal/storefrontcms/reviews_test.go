package storefrontcms_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

func connectTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://techlane:techlane@localhost:5432/techlane?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
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
	t.Cleanup(pool.Close)
	return pool
}

// seedVerifiedPurchase creates a tenant/product/variant/customer and a sale
// order in the given status containing that variant, returning the IDs
// needed to test the review gate.
func seedOrderForReview(t *testing.T, pool *pgxpool.Pool, status string) (tenantID, productID, customerID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID = uuid.New()
	productID = uuid.New()
	variantID := uuid.New()
	customerID = uuid.New()
	orderID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO identity.tenants (id, name) VALUES ($1, 'Review Test Tenant')`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO inventory.products (id, tenant_id, name) VALUES ($1, $2, 'Reviewed Product')`, productID, tenantID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO inventory.product_variants (id, tenant_id, product_id, sku, sell_price, cost_price)
		VALUES ($1, $2, $3, 'REV-SKU', 1000, 500)`, variantID, tenantID, productID); err != nil {
		t.Fatalf("seed variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO repair.customers (id, tenant_id, full_name, email, password_hash)
		VALUES ($1, $2, 'Test Customer', $3, 'x')`, customerID, tenantID, customerID.String()+"@example.com"); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO sales.orders (id, tenant_id, customer_id, channel, status, subtotal, total)
		VALUES ($1, $2, $3, 'online', $4, 1000, 1000)`, orderID, tenantID, customerID, status); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO sales.order_items (id, tenant_id, order_id, variant_id, quantity, unit_price, line_total)
		VALUES ($1, $2, $3, $4, 1, 1000, 1000)`, uuid.New(), tenantID, orderID, variantID); err != nil {
		t.Fatalf("seed order item: %v", err)
	}
	return tenantID, productID, customerID
}

func TestCreateOrUpdateReview_RequiresDeliveredOrder(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	svc := storefrontcms.NewService(pool)

	tenantID, productID, customerID := seedOrderForReview(t, pool, "ready_for_pickup")
	if _, err := svc.CreateOrUpdateReview(ctx, tenantID, customerID, productID, 5, "Great", "Loved it"); err == nil {
		t.Fatal("expected review to be rejected for a not-yet-delivered order")
	}

	nonPurchaser := uuid.New()
	if _, err := svc.CreateOrUpdateReview(ctx, tenantID, nonPurchaser, productID, 5, "Great", "Loved it"); err == nil {
		t.Fatal("expected review to be rejected for a non-purchaser")
	}
}

func TestCreateOrUpdateReview_DeliveredOrderSucceedsAndAggregates(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	svc := storefrontcms.NewService(pool)

	tenantID, productID, customerID := seedOrderForReview(t, pool, "delivered")
	review, err := svc.CreateOrUpdateReview(ctx, tenantID, customerID, productID, 4, "Solid", "Works well")
	if err != nil {
		t.Fatalf("CreateOrUpdateReview: %v", err)
	}
	if review.Rating != 4 || review.Status != "published" {
		t.Fatalf("unexpected review: %+v", review)
	}

	// Resubmitting updates in place rather than creating a duplicate.
	if _, err := svc.CreateOrUpdateReview(ctx, tenantID, customerID, productID, 2, "Changed my mind", "Meh"); err != nil {
		t.Fatalf("update review: %v", err)
	}

	published, err := svc.ListProductReviews(ctx, tenantID, productID)
	if err != nil {
		t.Fatalf("ListProductReviews: %v", err)
	}
	if len(published) != 1 {
		t.Fatalf("expected exactly 1 review after update, got %d", len(published))
	}
	if published[0].Rating != 2 {
		t.Fatalf("expected updated rating 2, got %d", published[0].Rating)
	}

	summaries, err := svc.ProductRatingSummaries(ctx, tenantID, []uuid.UUID{productID})
	if err != nil {
		t.Fatalf("ProductRatingSummaries: %v", err)
	}
	sum, ok := summaries[productID]
	if !ok {
		t.Fatal("expected a rating summary for the product")
	}
	if sum.Count != 1 || sum.Average != 2 {
		t.Fatalf("expected avg=2 count=1, got %+v", sum)
	}

	// Hiding a review removes it from the published list and the summary.
	if err := svc.SetReviewStatus(ctx, tenantID, published[0].ID, "hidden"); err != nil {
		t.Fatalf("SetReviewStatus: %v", err)
	}
	afterHide, err := svc.ListProductReviews(ctx, tenantID, productID)
	if err != nil {
		t.Fatalf("ListProductReviews after hide: %v", err)
	}
	if len(afterHide) != 0 {
		t.Fatalf("expected 0 published reviews after hiding, got %d", len(afterHide))
	}
}

func TestRecordView_IncrementsAndRanksMostViewed(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	svc := storefrontcms.NewService(pool)

	tenantID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO identity.tenants (id, name) VALUES ($1, 'View Test Tenant')`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	popular, unpopular := uuid.New(), uuid.New()

	for i := 0; i < 3; i++ {
		if err := svc.RecordView(ctx, tenantID, popular); err != nil {
			t.Fatalf("RecordView popular: %v", err)
		}
	}
	if err := svc.RecordView(ctx, tenantID, unpopular); err != nil {
		t.Fatalf("RecordView unpopular: %v", err)
	}

	top, err := svc.TopViewedVariantIDs(ctx, tenantID, 5)
	if err != nil {
		t.Fatalf("TopViewedVariantIDs: %v", err)
	}
	if len(top) != 2 || top[0] != popular {
		t.Fatalf("expected [popular, unpopular] most-viewed order, got %v", top)
	}
}
