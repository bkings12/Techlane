package platform

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/packages/pkg/migrate"
)

// Order matters: repair/003_job_code.sql references audit.risk_alerts and
// inventory.supplier_issues, so audit-risk and inventory-supplier must be
// migrated before repair on a fresh database.
var migrationDirs = []string{
	"services/identity/migrations",
	"services/audit-risk/migrations",
	"services/inventory-supplier/migrations",
	"services/payments-cash/migrations",
	"services/repair/migrations",
	"services/pos-sales/migrations",
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, repoRoot string) error {
	for _, dir := range migrationDirs {
		abs := filepath.Join(repoRoot, dir)
		if err := migrate.RunDir(ctx, pool, abs); err != nil {
			return fmt.Errorf("migrations %s: %w", dir, err)
		}
	}
	return nil
}
