# TechLane storefront (Phase 5)

Public buy-online-collect-in-branch shop on the shared platform APIs.

```bash
cd apps/web-storefront
npm install
npm run dev   # http://localhost:5174
```

Requires platform on `:8080`.

## Public APIs

- `GET /api/v1/commerce/public/bootstrap`
- `GET /api/v1/commerce/public/catalog`
- `POST /api/v1/commerce/public/checkout`
- `GET /api/v1/commerce/public/orders/{id}`

Flow: browse → C2B checkout (stock reserved) → paybill with `ORD-…` ref → collection code → branch pickup (ops/Android).
