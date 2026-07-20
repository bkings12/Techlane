# API Strategy

## Style

- **External:** REST JSON over HTTPS, versioned `/api/v1`
- **Internal:** REST by default; gRPC only for hot path inventory reserve/commit
- **Contracts:** OpenAPI 3.1 in `contracts/openapi/`; generate TypeScript and Kotlin clients
- **Errors:** Structured problem+json style envelope

## Error envelope

```json
{
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Not enough available quantity",
    "details": [{"field": "variant_id", "reason": "..."}],
    "correlation_id": "uuid",
    "docs_url": null
  }
}
```

## Authentication

- Access token: short-lived JWT (e.g. 15m) with `sub`, `tenant_id`, `roles`, `branch_ids`, `permissions`
- Refresh token: rotated, stored hashed, revocable
- Android: secure storage; device_id claim after registration
- Webhooks: HMAC signature headers; no JWT

## Authorization

- Gateway authenticates; services authorize using claims + local policy
- Never trust client-sent role, branch, price, tax, or totals
- Branch scope enforced server-side

## Idempotency

- Clients send `Idempotency-Key` on POST/PUT for money, inventory, sync commands
- Gateway/Redis stores key → response for TTL (e.g. 24h)
- Domain services also key critical writes by `action_id` for offline commands

## Pagination & filtering

- Cursor or offset+limit; prefer cursor for large lists
- Server-side filter/sort for all tables
- Max page size enforced

## Correlation

- `X-Correlation-ID` accepted or generated; propagated to logs, events, audit

## Webhooks (inbound)

- M-Pesa, bank paybill callbacks hit payments-cash via gateway path with signature verify
- Raw payload stored for replay/audit

## Client generation

- `tools/codegen` produces `@techlane/api-client` (TS) and Android Retrofit/Ktor models
- Do not hand-maintain divergent DTOs

## Versioning

- Breaking changes → `/api/v2`
- Additive fields allowed in v1 with care
- Deprecation window documented

## BFF

- Gateway may aggregate owner dashboard counts
- Domain rules stay in domain services — BFF does not recalculate prices or stock
