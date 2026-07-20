# Security Model

## Principles

1. Never trust UI-submitted role, branch, amount, price, discount, tax, or totals.
2. Recalculate and authorize everything in Go.
3. Least privilege; branch-scoped access.
4. Defense in depth: gateway + service policies + DB tenant filters.
5. Secrets outside source control.

## Authentication

- Short-lived access JWT + rotating refresh tokens
- Refresh tokens hashed at rest; reuse detection revokes family
- Account lockout after repeated failures
- Session revocation on logout / admin disable / device revoke
- Android: encrypted token storage; registered `device_id`

## Authorization

- RBAC with fine-grained permissions (see [roles-and-permissions.md](roles-and-permissions.md))
- Branch membership required for branch resources
- Manager approval workflow for sensitive actions (refunds, price overrides, adjustments, handover confirm)

## Transport & data

- TLS everywhere
- Encrypt sensitive fields at rest where appropriate (e.g. optional national IDs)
- PII minimization in logs
- Object storage private buckets + pre-signed URLs

## API protection

- Rate limiting (Redis)
- Idempotency keys on mutating financial/inventory endpoints
- Webhook HMAC verification
- Request size limits
- CORS locked to known web origins

## Audit

- Every sensitive mutation emits audit event (actor, role, branch, device, IP, action, entity, before/after, reason, correlation_id)
- Audit append-only; ordinary users cannot edit/delete

## Payment security

- Employees cannot mark digital payments paid by typing a reference alone
- Confirmation comes from provider webhook / query API
- Separation of payment confirmation vs settlement/allocation

## Cash security

- Cash tied to employee/drawer
- Handover cannot be self-approved
- Shortages create auditable discrepancies

## Device & mobile

- Device registration before sync
- Offline commands signed with user session; rejected if session revoked
- Root/jailbreak detection optional later; do not rely on it for security

## Secrets

- Env / secret manager; `.env.example` without secrets
- Rotate JWT signing keys with kid support

## Threat focus (leakage)

- Orphan supplier parts without jobs
- Unrecorded cash repairs
- Fake M-Pesa refs
- Self-approved handovers
- Silent inventory edits
