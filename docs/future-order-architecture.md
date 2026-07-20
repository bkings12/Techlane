# Future Order Architecture

## Status lifecycle

`draft` → `pending_payment` → `payment_processing` → `confirmed` → `preparing` → `ready_for_pickup` → `dispatched` → `delivered`  
Also: `partially_fulfilled`, `cancelled`, `returned`, `partially_refunded`, `fully_refunded`, `failed`

## Order model (Phase 5+)

- Order header: customer, channel, branch fulfilment, addresses, fees, taxes, discounts
- Order items: variant_id, qty, prices, fulfilment status
- Split payments via payments-cash allocations
- Partial fulfilment and partial returns supported

## Relationship to POS sales

Phase 3 `sales` / `sale_items` cover in-store. Either:

1. Keep `sales` for POS and add `orders` for online, both allocating payments and inventory the same way, or  
2. Unify under `orders` with `channel=pos|online`

**Preferred long-term:** unify under orders/commerce service with `channel`. MVP POS may use `sales` table then migrate/alias — document migration in Phase 5.

## Omnichannel workflows

### Buy online, collect in branch
Reserve branch stock → pay → prepare → collection code → verify → hand over → reconcile

### View online, pay in shop
Cart → quotation/pending order → cashier retrieves → pay in shop → complete

### Repair upsell
Offer accessory on job → add to repair invoice or linked sale/order

### Out-of-stock branch
Transfer or alternate fulfilment → notify customer

## Payments

Same payments-cash service; allocation `payable_type=order`.

## Extraction timing

Create Commerce/Order service when online checkout begins — not before. Until then, keep event names reserved.
