# Commerce Readiness

## Principle

Repair shop, POS, branch inventory, and future online shop share one platform:

**One product catalog · One inventory truth · One customer identity · One payment platform · One order/sales history · Multiple selling channels**

## Design now (without building storefront)

| Design now | Why |
|------------|-----|
| Product / variant / SKU / barcode / specs JSON | Avoid dual catalog later |
| Stock locations + balances (available/reserved/…) | Channel-safe stock |
| `inventory_reservations` with expiry | Prevent overselling |
| Unified `customers` | Repair + POS + online |
| Payments with generic allocation targets | `repair_id`, `sale_id`, `order_id` |
| `channel` on sales | pos / online / social / … |
| Nullable `merchant_id` / `seller_id` on catalog/stock | Marketplace later without rewrite |
| Pricing fields: list, cost, branch overrides table stub | Centralize price validation |
| Events reserved in catalog (marked later-phase) | Contract stability |

## Postpone

- Public storefront UI
- Carts, checkout UX, delivery, promotions, reviews
- Dedicated search cluster
- Marketplace commissions/payouts
- Customer shopping Android app

## Services that support e-commerce

| Current | Support |
|---------|---------|
| inventory-supplier | Canonical catalog + stock + reservations |
| payments-cash | All channels’ payments |
| repair | Unified customers; upsell hooks |
| pos-sales | Channel sales; precursor to orders |
| identity | Customer accounts (portal) later |
| notification | Order updates later |
| audit-risk | Commerce audit later |

## Future extraction

**Commerce / Order Service** owns: carts, orders, fulfilment, delivery, returns, promotions, coupons.

Keep catalog+inventory in inventory-supplier until product volume justifies a Product Service split. Avoid two competing product models.

## POS ↔ e-commerce inventory

Same `variant_id` and `stock_location_id`. Online checkout creates reservation → payment confirm converts to movement/sale → expiry releases. POS decrements available via same ledger.

## Unified customer

Single `customers` table (repair schema or future CRM extract). Portal shows repairs + purchases + orders.

## Payment reuse

No separate e-commerce payment stack. Allocations reference payable type + id.

## Oversell prevention

Reservations with TTL; available = physical − reserved − holds; transactional reserve.

## Marketplace later

Optional ownership fields; do not force commission tables into MVP. Document extension points only.

## Expensive mistakes to avoid

1. Separate online SKU table  
2. Separate customer tables per channel  
3. Separate payment services  
4. Hardcoded phone columns on products  
5. Subtracting stock only in UI  
6. Last-write-wins sync on stock  
7. Building marketplace tables before need  

## Related

- [product-catalog-model.md](product-catalog-model.md)
- [omnichannel-inventory.md](omnichannel-inventory.md)
- [future-order-architecture.md](future-order-architecture.md)
- [commerce-roadmap.md](commerce-roadmap.md)
