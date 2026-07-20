# Event Catalog

Events are versioned (`v1`). Consumers must be idempotent. Failed processing goes to DLQ after bounded retries.

## Envelope

```json
{
  "event_id": "uuid",
  "event_type": "repair.created",
  "event_version": 1,
  "occurred_at": "ISO-8601",
  "tenant_id": "uuid",
  "branch_id": "uuid|null",
  "correlation_id": "uuid",
  "actor_id": "uuid|null",
  "payload": {}
}
```

---

## Identity

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `user.created` | 1 | identity | audit, notify |
| `user.deactivated` | 1 | identity | audit, gateway (session revoke) |
| `session.revoked` | 1 | identity | gateway |
| `device.registered` | 1 | identity | audit |

## Repair

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `customer.created` | 1 | repair | audit |
| `device.registered` | 1 | repair | audit |
| `repair.created` | 1 | repair | audit, notify |
| `repair.assigned` | 1 | repair | audit, notify |
| `repair.status_changed` | 1 | repair | audit, notify, risk |
| `repair.completed` | 1 | repair | audit, notify, risk |
| `repair.collected` | 1 | repair | audit, notify |

## Inventory & supplier

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `part.requested` | 2 | inventory | audit, notify |
| `part.approved` | 2 | inventory | audit, notify |
| `part.collected` | 2 | inventory | audit, repair, risk, payments (credit) |
| `inventory.moved` | 2 | inventory | audit, risk, sales |
| `inventory.reserved` | 5 | inventory | audit |
| `inventory.reservation_expired` | 5 | worker | audit, commerce |
| `supplier.credit_updated` | 2 | inventory | audit, risk, reporting |
| `product.created` | 3 | inventory | audit |
| `product.updated` | 3 | inventory | audit |
| `product.published` | 5 | inventory | storefront search later |
| `product.price_changed` | 5 | inventory | audit, storefront |

## Sales / POS

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `sale.created` | 3 | pos-sales | audit |
| `sale.completed` | 3 | pos-sales | audit, inventory, payments |
| `sale.reversed` | 3 | pos-sales | audit, inventory, payments |

## Payments & cash

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `payment.initiated` | 3 | payments | audit |
| `payment.confirmed` | 3 | payments | audit, repair, sales |
| `payment.failed` | 3 | payments | audit, notify |
| `payment.allocated` | 3 | payments | audit, repair, sales |
| `payment.reversed` | 3 | payments | audit, repair, sales |
| `cash.received` | 2–3 | payments | audit, risk |
| `cash.handover_requested` | 3 | payments | audit, notify |
| `cash.handover_confirmed` | 3 | payments | audit |
| `cash.shortage_recorded` | 3 | payments | audit, risk |

## Audit & risk

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `risk.alert_raised` | 2 | audit-risk | notify, reporting |
| `risk.alert_resolved` | 2 | audit-risk | notify |

## Sync

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `sync.command_received` | 4 | gateway | audit |
| `sync.command_applied` | 4 | domain | audit |
| `sync.command_rejected` | 4 | domain | audit, android via API |

## Notification

| Event | Phase | Publisher | Consumers |
|-------|-------|-----------|-----------|
| `notification.sent` | 1+ | notification | audit |
| `notification.failed` | 1+ | notification | audit, worker |

## Future commerce (later phase — do not implement yet)

| Event | Phase | Notes |
|-------|-------|-------|
| `cart.created` | 5 | |
| `checkout.started` | 5 | |
| `order.created` | 5 | |
| `order.confirmed` | 5 | triggers inventory conversion |
| `order.cancelled` | 5 | release reservation |
| `order.ready_for_pickup` | 5 | BOCP |
| `order.dispatched` | 6 | |
| `order.delivered` | 6 | |
| `return.requested` | 6 | |
| `return.approved` | 6 | |
| `refund.requested` | 6 | |
| `refund.completed` | 6 | |
| `promotion.activated` | 6 | |
| `promotion.expired` | 6 | |

## Routing

- Exchange: `techlane.events` (topic)
- Routing key: event type (e.g. `repair.completed`)
- DLQ: `techlane.events.dlq`
