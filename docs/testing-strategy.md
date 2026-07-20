# Testing Strategy

## Layers

| Layer | Focus |
|-------|-------|
| Unit | Domain rules: status machines, allocation, stock math, permissions |
| Integration | Service + Postgres + Redis |
| Contract | OpenAPI consumer/provider |
| Event consumer | Idempotent handlers, DLQ |
| Payment webhook | Signature, replay, confirm/fail paths |
| Offline sync | Duplicate action_id, payload mismatch, conflicts |
| Permission | Role matrix |
| E2E | Critical workflows (API and/or Playwright + Android instrumentation) |

## Critical workflows (must automate)

1. Create repair and assign technician  
2. Request and approve supplier screen  
3. Collect the screen  
4. Complete the repair  
5. Receive cash  
6. Hand over cash  
7. Confirm digital payment  
8. Reconcile supplier credit  
9. Detect part with no matching repair  
10. Prevent duplicate offline synchronization  
11. Reverse a payment safely  
12. Transfer stock between branches  
13. Complete a retail sale  
14. Process a warranty return  

## Financial / inventory rules

- No silent updates to completed payments  
- Movements append-only  
- Reservations expire correctly  
- Handover self-approval rejected  

## Tooling

- Go: `testing`, testcontainers where useful  
- Web: Vitest + Playwright  
- Android: JUnit + Espresso/Compose tests  
- CI: run unit + integration on PR; E2E nightly or on main  

## Definition of done for money features

Workflows 5–11 and 13 green in CI before production Phase 3.
