# Database Ownership

## Principles

1. PostgreSQL is the source of truth.
2. Each service owns its schema (logical DB-per-service in one instance initially).
3. No cross-service writable shared tables.
4. Cross-service references use UUIDs only (soft FKs).
5. Public IDs are UUIDs — never expose sequential internals.
6. Standard columns on important records: `id`, `tenant_id`, `branch_id` (where applicable), `created_at`, `updated_at`, `created_by`, `updated_by`, `version`, `status`, `source_device_id`, `correlation_id`.
7. Financial and inventory ledgers are append-only; corrections via new records.

## Schema map

| Schema | Service | Owns |
|--------|---------|------|
| `identity` | identity-service | tenants, users, roles, permissions, branches, memberships, sessions, devices |
| `repair` | repair-service | customers, devices, jobs, diagnoses, estimates, status events, warranties |
| `inventory` | inventory-supplier-service | products, variants, locations, balances, movements, reservations, suppliers, part requests, supplier issues, credit |
| `sales` | pos-sales-service | sales, sale_items |
| `payments` | payments-cash-service | payments, allocations, cash, handovers, mpesa, bank, refunds |
| `audit` | audit-risk-service | audit_events, risk_alerts |
| `notify` | notification-service | notification_outbox |
| `gateway` | api-gateway | sync_commands (MVP) |

## Entity ownership detail

### identity
- `tenants`, `branches`, `users`, `roles`, `permissions`, `role_permissions`
- `branch_memberships` (user ↔ branch ↔ roles)
- `sessions`, `refresh_tokens`, `registered_devices`

### repair
- `customers` — unified customer identity (POS/commerce reuse by ID)
- `devices` — IMEI/serial, customer link or anonymous flag
- `repair_jobs`, `repair_diagnoses`, `repair_estimates`
- `repair_status_events` — immutable timeline
- `repair_attachments` — R2 keys
- `warranties` — repair-linked initially

### inventory
- `categories`, `brands`, `products`, `product_variants` (SKU, barcode, specs JSON)
- `stock_locations` (branch, warehouse, technician-held, damaged, in-transit, etc.)
- `inventory_balances` — physical, available, reserved, damaged, in_transit
- `inventory_movements` — append-only ledger with reason codes
- `inventory_reservations` — with expiry (commerce-ready)
- `suppliers`, `part_requests`, `supplier_issues`, `supplier_credit_entries`
- Optional nullable `merchant_id` / `seller_id` for future marketplace

### sales
- `sales` — channel (`pos`, `online`, …), branch, customer
- `sale_items` — variant_id, qty, prices (server-calculated)

### payments
- `payments` — method, status lifecycle (initiated→pending→confirmed→allocated…)
- `payment_allocations` — generic payable refs (`repair_id`, `sale_id`, `order_id`, …)
- `cash_drawers`, `cash_ledger_entries`, `cash_handovers`
- `mpesa_transactions`, `bank_transactions`, `refunds`, `reversals`

### audit
- `audit_events` — append-only, protected
- `risk_alerts` — orphan parts, shortages, unverified payments

### gateway
- `sync_commands` — offline outbox ingest with idempotency
- `idempotency_records` may live in Redis with DB backup for critical ops

## Multi-tenancy

All business tables include `tenant_id`. Queries always filter by tenant from the authenticated token — never from client-supplied body alone.

## Migrations

- golang-migrate or goose per service
- Migrations owned by the service; never alter another service’s schema

## Related

- [product-catalog-model.md](product-catalog-model.md)
- [omnichannel-inventory.md](omnichannel-inventory.md)
- [offline-sync-strategy.md](offline-sync-strategy.md)
