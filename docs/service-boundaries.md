# Service Boundaries

## Design rules

- Not one service per table
- Prefer events for fan-out; prefer sync request when the caller needs an immediate authoritative answer
- Avoid distributed transactions; use sagas/compensations for cross-service workflows
- Each service owns its schema and publishes events for others to react

---

## 1. API Gateway / BFF

| Aspect | Detail |
|--------|--------|
| **Responsibility** | TLS termination (deploy), JWT validation, routing, rate limiting, correlation IDs, idempotency key intake, light BFF aggregation for dashboards |
| **Data ownership** | Gateway config; Redis rate-limit/idempotency keys; `sync_commands` ingest metadata (MVP) |
| **Main entities** | IdempotencyRecord (Redis), SyncCommand (Postgres schema `gateway` or `sync`) |
| **Public APIs** | `/api/v1/*` reverse proxy; `/api/v1/sync/commands`; health |
| **Events published** | `sync.command_received` |
| **Events consumed** | None required |
| **Security** | Auth required except health/webhooks; device registration checks for Android |
| **Failure** | Upstream timeout → 504; circuit break optional later |
| **MVP** | **Required** |

---

## 2. Identity and Access Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Tenants, users, roles, permissions, branches, memberships, sessions, refresh rotation, device registration |
| **Data ownership** | `tenants`, `users`, `roles`, `permissions`, `role_permissions`, `branches`, `branch_memberships`, `sessions`, `registered_devices` |
| **Public APIs** | Login, refresh, logout, me, users CRUD, branches, roles, device register/revoke |
| **Events published** | `user.created`, `user.deactivated`, `session.revoked`, `device.registered` |
| **Events consumed** | None critical |
| **Security** | Password hashing, short-lived access tokens, refresh rotation, lockout |
| **Failure** | Auth outage blocks all clients |
| **MVP** | **Required** |

---

## 3. Repair Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Customers, devices, job cards, diagnoses, estimates, status timeline, collection proof, repair-linked warranties |
| **Data ownership** | `customers`, `devices`, `repair_jobs`, `repair_diagnoses`, `repair_estimates`, `repair_status_events`, `repair_attachments`, `warranties` |
| **Public APIs** | Customer/device/job CRUD, assign tech, status transitions, collection |
| **Events published** | `repair.created`, `repair.assigned`, `repair.status_changed`, `repair.completed`, `repair.collected`, `customer.created`, `device.registered` |
| **Events consumed** | `part.collected` (enrich job), `payment.allocated` (balance status) |
| **Security** | Branch-scoped access; sensitive status changes may require online |
| **Failure** | Inventory sync call for part request may fail → job remains without part |
| **MVP** | **Required** |

---

## 4. Inventory and Supplier Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Product catalog (commerce-ready), stock locations, balances, movements, reservations, suppliers, part requests, supplier issues/auth codes, supplier credit, transfers |
| **Data ownership** | `products`, `product_variants`, `categories`, `stock_locations`, `inventory_balances`, `inventory_movements`, `inventory_reservations`, `suppliers`, `part_requests`, `supplier_issues`, `supplier_credit_ledgers` |
| **Public APIs** | Catalog, stock, movements, part request/approve/collect, suppliers, credit, transfers |
| **Events published** | `part.requested`, `part.approved`, `part.collected`, `inventory.moved`, `inventory.reserved`, `inventory.reservation_expired`, `supplier.credit_updated`, `product.created`, `product.updated` |
| **Events consumed** | `repair.created` (optional), `order.confirmed` (later), `sale.completed` |
| **Security** | Approvals for adjustments; auth codes for supplier collection |
| **Failure** | Reservation expiry via worker; never silent stock overwrite |
| **MVP** | **Required** (catalog subset + part flow) |

---

## 5. POS and Sales Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | POS sales, sale items, channel sales; later thins when Commerce/Order extracted |
| **Data ownership** | `sales`, `sale_items` |
| **Public APIs** | Create/complete sale, void with approval, list sales |
| **Events published** | `sale.created`, `sale.completed`, `sale.reversed` |
| **Events consumed** | `payment.allocated`, `inventory.moved` |
| **Security** | Price/discount server-validated; manager approval for voids |
| **Failure** | Inventory reserve failure aborts sale |
| **MVP** | **Phase 3** |

---

## 6. Payments and Cash Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Payments, allocations, M-Pesa/bank verification, cash drawers, employee cash balances, handovers, refunds/reversals |
| **Data ownership** | `payments`, `payment_allocations`, `cash_drawers`, `cash_ledger`, `cash_handovers`, `mpesa_transactions`, `bank_transactions`, `refunds` |
| **Public APIs** | Initiate/confirm payment, webhooks, cash receive, handover create/confirm, refunds |
| **Events published** | `payment.initiated`, `payment.confirmed`, `payment.failed`, `payment.allocated`, `payment.reversed`, `cash.received`, `cash.handover_requested`, `cash.handover_confirmed`, `cash.shortage_recorded` |
| **Events consumed** | Webhook-driven; `sale.completed`, `repair.completed` for allocation context |
| **Security** | Webhook signature verification; no self-approval of handovers |
| **Failure** | Pending until confirmed; never employee-typed ref alone |
| **MVP** | Thin cash recording in vertical slice; full verify **Phase 3** |

---

## 7. Notification Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | SMS, WhatsApp, email, push outbox |
| **Data ownership** | `notification_outbox`, `notification_templates` |
| **Public APIs** | Admin template list; internal send API |
| **Events published** | `notification.sent`, `notification.failed` |
| **Events consumed** | Repair/payment/order status events |
| **Security** | Internal-only send; PII minimization in logs |
| **Failure** | Retry with backoff; DLQ |
| **MVP** | **Thin** (log/outbox only acceptable) |

---

## 8. Audit and Risk Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Append-only audit; risk alerts (orphan parts, shortages, stuck jobs) |
| **Data ownership** | `audit_events`, `risk_alerts` |
| **Public APIs** | Query audit (authorized), list/ack alerts |
| **Events published** | `risk.alert_raised`, `risk.alert_resolved` |
| **Events consumed** | All domain events; explicit audit commands |
| **Security** | Read restricted; no update/delete for ordinary users |
| **Failure** | Audit write failure must not silently drop — durable queue |
| **MVP** | **Required** |

---

## 9. Reporting and Analytics Service

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Aggregates, owner reports, branch comparison |
| **Data ownership** | Read models / materialized views (later) |
| **Public APIs** | Report endpoints |
| **Events consumed** | Domain events for projections |
| **MVP** | **Later** (Phase 4); MVP dashboards via BFF queries |

---

## 10. Worker / Scheduler

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Retries, DLQ processing, reservation expiry, reconciliation jobs, orphan-part scans |
| **Data ownership** | Job schedules / lease state |
| **Events published** | `inventory.reservation_expired`, reconciliation events |
| **MVP** | **Required** |

---

## 11. Mobile Synchronization (MVP placement)

| Aspect | Detail |
|--------|--------|
| **Responsibility** | Offline command ingest, idempotent dispatch to domain services |
| **MVP** | Implemented in **gateway + domain services** (shared idempotency package) |
| **Later** | Extract dedicated `mobile-sync-service` if volume warrants |

---

## 12. Future Commerce / Order Service

Extracted in Phase 5+: carts, orders, fulfilment, promotions. Catalog and inventory remain in inventory-supplier until/unless product service splits.

See [commerce-readiness.md](commerce-readiness.md).
