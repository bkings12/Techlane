# Omnichannel Inventory

## Stock locations

Examples:

- Main shop / branch floor
- Warehouse
- Technician-held
- Supplier-reserved
- Damaged
- Returned
- In-transit

Each location has `tenant_id`, type, optional `branch_id`.

## Quantity dimensions

| Dimension | Meaning |
|-----------|---------|
| physical | On-hand count |
| available | Sellable / issuable |
| reserved | Held for checkout/orders |
| in_transit | Between locations |
| damaged | Not sellable |

Sold quantity is historical via movements/sales — not a balance field that is edited.

## Movements (append-only)

Every change creates an `inventory_movement` with reason:

- repair_issue, stock_purchase, branch_transfer, warranty_replacement, internal_use, supplier_return, damaged, sale, reservation_convert, adjustment (approved)

Part leaving without reason is forbidden.

## Reservations

```
checkout started → reserve (TTL)
payment confirmed → convert reservation → sale movement
fail/expire → release reservation
```

POS and online must not sell the same unit: serialize with row locks / conditional updates on available qty.

## Transfers

Create in-transit movement pair; complete online; audit trail.

## Supplier-issued parts

Supplier issue records link to repair job + auth code; may create movement into technician-held or directly consume against job depending on policy (document in ops). Prefer: collect → movement with reason `repair_issue` tied to `supplier_issue_id` + `repair_job_id`.

## Reconciliation

Worker jobs: expired reservations, orphan supplier issues, negative available (should be impossible — alert if detected).
