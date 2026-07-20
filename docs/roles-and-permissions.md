# Roles and Permissions

## Roles (MVP)

| Role | Description |
|------|-------------|
| `owner` | Full access within tenant |
| `manager` | Branch ops, approvals, overrides |
| `accountant` | Payments, reconciliations, reports (read financial) |
| `inventory` | Stock, suppliers, part approvals |
| `technician` | Assigned jobs, diagnoses, part requests |
| `cashier` | POS, repair payments, cash receive |
| `branch_admin` | Local user/branch settings |

Future: `supplier`, `customer` (portal-scoped).

## Permission catalog (representative)

### Identity
- `users.read`, `users.write`, `roles.assign`, `devices.manage`

### Branches
- `branches.read`, `branches.write`

### Customers & devices
- `customers.read`, `customers.write`, `devices.read`, `devices.write`

### Repairs
- `repairs.read`, `repairs.create`, `repairs.assign`, `repairs.status.update`, `repairs.price.override`, `repairs.collect`

### Inventory
- `inventory.read`, `inventory.adjust`, `inventory.transfer`, `parts.request`, `parts.approve`, `parts.collect`, `suppliers.read`, `suppliers.write`, `supplier_credit.reconcile`

### Sales
- `sales.create`, `sales.read`, `sales.void`

### Payments & cash
- `payments.initiate`, `payments.read`, `cash.receive`, `cash.handover.request`, `cash.handover.confirm`, `refunds.create`, `refunds.approve`

### Audit & risk
- `audit.read`, `risk.read`, `risk.ack`

### Reports
- `reports.read`

## Role → permission defaults

| Permission | owner | manager | accountant | inventory | technician | cashier | branch_admin |
|------------|-------|---------|------------|-----------|------------|---------|--------------|
| users.write | Y | Y* | | | | | Y* |
| repairs.create | Y | Y | | | Y | Y | |
| repairs.assign | Y | Y | | | | | |
| parts.approve | Y | Y | | Y | | | |
| parts.collect | Y | Y | | Y | Y | | |
| cash.handover.confirm | Y | Y | Y | | | | |
| cash.handover.request | Y | Y | | | | Y | |
| refunds.approve | Y | Y | Y | | | | |
| audit.read | Y | Y | Y | | | | |
| inventory.adjust | Y | Y | | Y | | | |

\* branch-scoped where applicable

## Rules

- Permissions enforced in backend only
- A user may belong to multiple branches with possibly different roles
- Self-approval forbidden for: cash handover confirm, own refund, own inventory adjustment approval
- Owner may collect parts; still requires documented job/auth code

## Custom roles and permissions

Owners and managers with `roles.write` can:

- Create custom roles with a unique key (e.g. `senior_tech`)
- Assign any subset of the permission catalog to a role
- Edit permissions on system roles (except `owner`, which always has full access)
- Delete custom roles that are not assigned to users
- Add custom permission codes to the catalog (for future feature gates)

Permission resolution at login loads from `identity.role_permissions`, not only hardcoded defaults.

### Commission permissions

| Permission | owner | manager | accountant |
|------------|-------|---------|------------|
| commissions.read | Y | Y | Y |
| commissions.write | Y | Y | |
| commissions.approve | Y | Y | Y |

Technicians cannot edit their own commission configuration.

Commission accrues when a repair is marked `completed` if the assigned technician has `commission_enabled`. Base amount is `labor_amount` on the job.
