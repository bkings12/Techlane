package platform

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/techlane/techlane/packages/pkg/migrate"
)

var migrationDirs = []string{
	"services/identity/migrations",
	"services/repair/migrations",
	"services/inventory-supplier/migrations",
	"services/payments-cash/migrations",
	"services/audit-risk/migrations",
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
