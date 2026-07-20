# MVP Scope

## In scope

### Backend
- Identity: login, refresh, RBAC, branches, device registration
- **Staff admin:** create/list/update users, assign roles & branches, role permission catalog
- **Optional commissions:** per-technician percent or fixed accrual on repair completed; approve / mark-paid
- Repair: customers, devices, jobs, assignment, status timeline, collection proof, labor_amount
- Inventory/supplier: products/variants (basic), stock locations, part request → approve → auth code → collect, supplier credit ledger entries
- Payments: provisional cash recording; payment status model; allocation to repair (full M-Pesa verify can land early Phase 3)
- Audit + risk: append-only audit; orphan-part alerts
- Gateway: auth, routing, correlation, idempotency
- Worker: orphan scan, retries
- Notification: outbox stub

### Web ops
- Login, role dashboards (owner/tech/cashier shells)
- **Settings:** staff, roles matrix, commissions
- Repair list/detail/intake
- Part request/approval views
- Risk alerts (orphan parts)
- Design tokens + core primitives

### Android
- Login, job queue/detail, intake, part request, online sync of drafts
- Room outbox skeleton
- Camera photo attach (upload when online)

## Out of scope (MVP)

- Full POS retail checkout
- Full M-Pesa C2B/paybill production hardening (stub interfaces OK)
- Cash handover full workflow (design + schema yes; complete UI Phase 3)
- Reporting warehouse
- Customer portal production
- Supplier portal
- Public e-commerce
- Kubernetes
- Search cluster
- Marketplace

## Vertical slice acceptance

1. Authenticated user creates customer + device + repair at a branch
2. Technician assigned; status history recorded
3. Part requested, approved, auth code issued, collection recorded against job
4. Repair completed; provisional cash recorded
5. Audit entries exist for each step
6. Owner sees alert if supplier issue lacks matching completed/paid job path

## Definition of done (MVP)

- Docs complete
- Compose stack runs locally
- Critical path API + web + Android smoke
- Tests for idempotency and part-job linkage
- No silent inventory/payment edits
