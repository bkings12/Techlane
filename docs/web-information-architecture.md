# Web Information Architecture

## App

`apps/web-ops` — React + Vite SPA for authenticated staff.

## Navigation model

Primary sidebar (role-filtered):

1. Home (role dashboard)
2. Repairs
3. Customers
4. Inventory
5. Suppliers
6. POS / Sales
7. Payments & Cash
8. Risk & Audit
9. Reports
10. Settings (users, branches, roles)

Top bar: branch selector (if multi-branch), search, notifications, user menu, online status.

## Route map (MVP → Phase 3)

| Route | Purpose |
|-------|---------|
| `/login` | Auth |
| `/` | Role dashboard |
| `/repairs` | Job list + filters |
| `/repairs/new` | Intake wizard |
| `/repairs/:id` | Job detail: timeline, parts, payments, drawer actions |
| `/customers` | Customer list |
| `/customers/:id` | Profile, devices, history |
| `/inventory` | Stock by location |
| `/inventory/movements` | Ledger |
| `/suppliers` | Suppliers |
| `/suppliers/:id` | Credit + issues |
| `/parts/requests` | Part request queue |
| `/pos` | POS workspace |
| `/payments` | Payment list |
| `/cash/handovers` | Handover queue |
| `/risk` | Alerts |
| `/audit` | Audit search |
| `/reports` | Phase 4 |
| `/settings/*` | Users, branches, permissions |

## Key wireframes (text)

### Owner home
```
[Branch: All ▾]                    [Search] [Alerts 3]

Needs attention
  • 2 parts without jobs
  • 1 cash shortage
  • 4 unverified payments

Today
  Revenue  |  Jobs open  |  Supplier debt

[Risk alerts list — compact]
```

### Repair detail
```
Job #…  [Status badge]  Branch  Technician
Device IMEI / Customer

[Primary actions: Request part | Update status | Take payment | Collect]

Tabs: Overview | Timeline | Parts | Payments | Files | Audit
```

### POS
```
Left: cart / line items
Right: customer lookup, pay methods, totals (server), complete
```

## Customer portal (separate app later)

Routes: status lookup, job detail (limited), receipts, warranty — no internal ops chrome.
