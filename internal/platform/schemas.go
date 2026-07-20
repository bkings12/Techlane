package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SchemaIdentity  = "identity"
	SchemaRepair    = "repair"
	SchemaInventory = "inventory"
	SchemaSales     = "sales"
	SchemaPayments  = "payments"
	SchemaAudit     = "audit"
	SchemaNotify    = "notify"
	SchemaGateway   = "gateway"
)

var AllSchemas = []string{
	SchemaIdentity,
	SchemaRepair,
	SchemaInventory,
	SchemaSales,
	SchemaPayments,
	SchemaAudit,
	SchemaNotify,
	SchemaGateway,
}

const ensureSchemasSQL = `
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS repair;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS sales;
CREATE SCHEMA IF NOT EXISTS payments;
CREATE SCHEMA IF NOT EXISTS audit;
CREATE SCHEMA IF NOT EXISTS notify;
CREATE SCHEMA IF NOT EXISTS gateway;
`

func EnsureSchemas(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, ensureSchemasSQL); err != nil {
		return fmt.Errorf("ensure schemas: %w", err)
	}
	return nil
}
