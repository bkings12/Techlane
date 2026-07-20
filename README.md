# TechLane

Repair-shop operations, inventory, supplier credit, POS, payments, and multi-branch accountability platform.

**Stack:** Go monolith (`cmd/platform`) · React+Vite ops/portal/supplier · Kotlin Compose Android · PostgreSQL · Cloudflare R2 / MinIO

## Documentation

Start at [docs/product-requirements.md](docs/product-requirements.md) and [docs/architecture-overview.md](docs/architecture-overview.md).

## Quick start

```bash
docker compose -f deploy/docker-compose.yml up -d
```

See service READMEs under `services/` and `docs/deployment-strategy.md`.

## Web apps

| App | Path | Dev port | Notes |
|-----|------|----------|--------|
| Staff ops | `apps/web-ops` | Vite default (`5173`) | Owner/manager/tech desk |
| Customer portal | `apps/web-portal` | `5175` | OTP session + guest job-code lookup |
| Supplier portal | `apps/web-supplier` | `5176` | Quote queue, issue QR, credit |
| Storefront | `apps/web-storefront` | (see app README) | Public shopfront |

API default for portals: `http://localhost:8080/api/v1` (`VITE_API_BASE` to override).

```bash
# Customer
cd apps/web-portal && npm install && npm run dev

# Supplier (seeded: supplier@techlane.local / password)
cd apps/web-supplier && npm install && npm run dev

# Staff ops
cd apps/web-ops && npm install && npm run dev
```

## Repository layout

- `apps/` — web-ops, web-portal, web-supplier, web-storefront, android
- `services/` — Go microservices
- `packages/pkg/` — shared Go libraries
- `contracts/` — OpenAPI, events, proto
- `docs/` — architecture and product docs
- `design-tokens/` — shared semantic tokens
- `deploy/` — Docker Compose

## License

Proprietary — all rights reserved.
