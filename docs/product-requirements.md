# Product Requirements — TechLane Repair Operations Platform

## 1. Vision

TechLane is a repair-shop operations platform that connects repairs, inventory, supplier credit, POS, payments, employee accountability, and multi-branch management. The primary business problem is **revenue and inventory leakage**: parts collected from suppliers and payments received without recorded jobs.

Every important transaction must be end-to-end traceable:

Customer device → repair job → technician → part request → supplier authorization → collection → repair completion → payment → cash handover or digital confirmation → device collection.

## 2. Business context

The business repairs phones and laptops and sells phones, laptops, screens, batteries, chargers, cables, cases, accessories, repair parts, and other electronics.

Current footprint:

- Main shop and at least one branch
- Owner, two or more employees, technicians
- Employees (and owner) may collect parts from suppliers on weekly credit
- Customers pay via cash, M-Pesa, bank paybill, and potentially C2B

## 3. Goals

1. Make unrecorded repairs and orphan supplier parts difficult or impossible.
2. Provide role-appropriate tools for owners, managers, technicians, cashiers, and inventory staff.
3. Support offline-capable Android operations for daily shop work.
4. Remain e-commerce-ready without building the storefront in MVP.

## 4. User roles

| Role | Primary needs |
|------|---------------|
| Owner | Revenue, leakage alerts, supplier debt, cash shortages, branch comparison |
| Manager | Approvals, overrides, branch operations |
| Accountant | Payments, reconciliations, reports |
| Inventory staff | Stock, transfers, supplier issues |
| Technician | Assigned jobs, parts, diagnosis, progress |
| Cashier | Sales, repair payments, cash drawer, handovers |
| Branch admin | Branch users, local settings |
| Customer (portal) | Repair status, balances, receipts, warranty |
| Supplier (future) | Authorization confirmation, credit statements |

## 5. Applications

1. **Web ops** — owners, managers, accountants, inventory, branch admin
2. **Android APK** — technicians, cashiers, daily shop operations
3. **Customer portal** — lightweight repair progress, receipts, warranty
4. **Supplier portal** — future authorization interface
5. **Shared Go backend** — all business rules and data

## 6. Functional requirements (summary)

### Repairs
- Job card per repair; branch-scoped; customer or anonymous walk-in; assigned technician
- Status history timeline; no deletion of completed repairs; collection proof required

### Parts & inventory
- No part leaves inventory without a documented reason (repair, purchase, transfer, warranty, internal use, return, damaged)
- Supplier-issued parts link: supplier, job, requested/approved/collected by, timestamps, cost, branch, auth code, reconciliation status

### Payments & cash
- Cash, M-Pesa STK/C2B, bank paybill/transfer, card, store credit, split payments
- Backend verification for digital payments; no employee-typed ref that instantly marks paid
- Cash assigned to employee/drawer; handover requires second authorized person; shortages auditable

### Audit & risk
- Append-only audit with actor, role, branch, device, IP, action, entity, before/after, reason, correlation ID
- Fraud/risk alerts for orphan parts, unverified payments, cash shortages, stuck jobs

### Offline Android
- Low-risk drafts offline; sensitive actions require online verification
- Outbox sync with idempotent backend processing

## 7. Non-goals (MVP)

- Full public e-commerce storefront
- Marketplace / multi-vendor
- Kubernetes
- Dedicated search cluster
- Delivery fleet management
- Customer mobile shopping app

## 8. Success metrics

- % of supplier issues linked to a repair job (target: 100%)
- Time to detect orphan parts (target: minutes)
- Cash handover completion rate and shortage rate
- Unverified digital payment count trending to zero
- Repair cycle time and ready-but-uncollected devices

## 9. Related documents

- [mvp-scope.md](mvp-scope.md)
- [roles-and-permissions.md](roles-and-permissions.md)
- [implementation-roadmap.md](implementation-roadmap.md)
- [commerce-readiness.md](commerce-readiness.md)
