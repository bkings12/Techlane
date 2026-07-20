# Implementation Roadmap

## Phase 0 — Foundation

| | |
|--|--|
| **Goal** | Repo, docs, tokens, compose infra, shared packages |
| **User value** | Team can develop safely |
| **Backend** | Shared pkg stubs, migrate tooling |
| **Web** | Vite shell + tokens |
| **Android** | Project shell |
| **DB** | Postgres schemas created empty |
| **Events** | Catalog documented |
| **Tests** | CI placeholder |
| **Acceptance** | `docker compose up` health; docs present |
| **Deps** | None |
| **Risks** | Over-scaffolding |

## Phase 1 — Identity & repairs

| | |
|--|--|
| **Goal** | Login, branches, customers, devices, jobs, timeline, audit |
| **User value** | Traceable job cards |
| **Backend** | identity, repair, audit-risk, gateway |
| **Web** | Login, repair list/detail/intake, owner/tech home shells |
| **Android** | Login, job list/detail, intake |
| **DB** | identity + repair + audit schemas |
| **Events** | repair.*, user.*, audit consume |
| **Tests** | Authz, job status transitions |
| **Acceptance** | Create→assign→status change→audit visible |
| **Deps** | Phase 0 |
| **Risks** | Over-flexible status model |

## Phase 2 — Parts & supplier trace (anti-leakage)

| | |
|--|--|
| **Goal** | Part request/auth/collect; orphan alerts; credit ledger |
| **User value** | Leakage hard |
| **Backend** | inventory-supplier, worker orphan scan, risk alerts |
| **Web** | Part queues, supplier issue trace, risk page |
| **Android** | Request part, show auth code, collect confirm |
| **DB** | inventory schema |
| **Events** | part.*, inventory.moved, risk.alert_raised |
| **Tests** | Part must link to job; idempotent collect; orphan detection |
| **Acceptance** | Vertical slice complete |
| **Deps** | Phase 1 |
| **Risks** | Supplier UX friction vs control |

## Phase 3 — Money & POS

| | |
|--|--|
| **Goal** | Verified payments, cash handovers, POS, catalog |
| **User value** | Cash accountability + retail sales |
| **Backend** | payments-cash, pos-sales; M-Pesa STK verify |
| **Web** | POS, payments, handovers |
| **Android** | Pay flows, cash receive, handover request |
| **DB** | payments + sales |
| **Events** | payment.*, cash.*, sale.* |
| **Tests** | No typed-ref instant pay; handover self-approve blocked; reversal |
| **Acceptance** | Workflows 5–8, 11, 13 from test plan |
| **Deps** | Phase 2 |
| **Risks** | Provider sandbox instability |

## Phase 4 — Offline, risk, reporting

| | |
|--|--|
| **Goal** | Mature outbox sync; richer risk; basic reports |
| **User value** | Shop works offline safely |
| **Backend** | sync hardening, reporting projections |
| **Web** | Reports, conflict admin |
| **Android** | Full outbox, sync center |
| **Tests** | Duplicate sync prevention; conflict rejection |
| **Deps** | Phase 3 |
| **Risks** | Sync edge cases |

## Phase 5 — E-commerce foundation

| | |
|--|--|
| **Goal** | Public catalog, cart, checkout, online pay, branch pickup |
| **User value** | Online channel on same inventory |
| **Backend** | Commerce/order extract or module; reservations |
| **Web** | storefront app |
| **Deps** | Stable Phase 3–4 |
| **Risks** | Overselling if reservations weak |

## Phase 6 — Delivery, returns, promotions, customer app

## Phase 7 — Marketplace (only if justified)

## Suggested build order inside Phase 1–2

1. Identity & branch access  
2. Customer & device  
3. Repair job  
4. Technician assignment  
5. Repair timeline  
6. Part request  
7. Supplier auth  
8. Part collection  
9. Cash recording  
10. Audit trail  
11. Owner risk dashboard  
12. Android offline drafts  

See also [commerce-roadmap.md](commerce-roadmap.md).
