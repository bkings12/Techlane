# UX Principles

## Feel

Premium · Modern · Trustworthy · Calm · Intelligent · Spacious · Professional · Fast · Clear · Consistent

## Anti-patterns

Crowded dashboards · Metric soup · Unrelated card grids · Generic admin templates · Equal visual weight for all data · Developer forms without grouping

## Information hierarchy (three levels)

1. **Immediate action** — what to do now
2. **Important context** — who, branch, job, money at risk
3. **Detailed history** — timelines, audit, expansions

Use progressive disclosure. Expand for detail; do not dump everything on first paint.

## Role dashboards

### Owner
Prioritize: revenue, repairs, supplier debt, parts without matching jobs, cash shortages, unverified payments, jobs waiting too long, ready devices not collected, branch differences, high-risk alerts.

### Technician
Assigned jobs, waiting for action, parts requested/ready, testing queue, ready for pickup.

### Cashier
Current sale, repair payment, customer lookup, payment status, cash drawer, pending handovers.

## Patterns

- Clear page titles and section grouping
- Search, filters, saved views
- Status indicators + labels
- Timelines and activity history
- Contextual actions; drawers/detail panels
- Confirmation for risky actions
- Empty / loading / error / permission-denied / offline / sync states

## Forms

- Good defaults, searchable selectors, inline validation
- Auto-save drafts where safe
- Minimal mandatory fields; conditional fields
- Guided multi-step for intake; power-user fast path

### Repair intake steps
1. Customer → 2. Device → 3. Problem → 4. Photos → 5. Estimate → 6. Technician → 7. Confirm

## Accessibility

Keyboard nav · visible focus · SR labels · semantic HTML · contrast · large touch targets · reduced motion · accessible dialogs/tables · status text + color

## Platform split

Web and Android share tokens, status language, terminology — **not** identical layouts. No desktop tables stuffed into APK; use lists, sheets, scan flows on mobile.

## Performance UX

Optimistic UI only for low-risk drafts. Sensitive actions wait for server confirmation.
