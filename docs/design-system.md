# Design System

## Goals

Premium, calm, trustworthy, spacious, professional. Not crowded, noisy, or template-like.

Semantic tokens only — no hardcoded colors/spacing in components.

## Color tokens

### Dark

| Token | Value |
|-------|-------|
| `color.background.canvas` | `#08111F` |
| `color.background.surface` | `#0F1B2D` |
| `color.background.elevated` | `#162338` |
| `color.background.subtle` | `#122033` |
| `color.text.primary` | `#F8FAFC` |
| `color.text.secondary` | `#CBD5E1` |
| `color.text.muted` | `#94A3B8` |
| `color.border.default` | `#26364D` |
| `color.border.strong` | `#3B4F6B` |
| `color.action.primary` | `#2563EB` |
| `color.action.primaryHover` | `#1D4ED8` |
| `color.action.secondary` | `#0891B2` |
| `color.status.success` | `#059669` |
| `color.status.warning` | `#D97706` |
| `color.status.danger` | `#DC2626` |
| `color.status.info` | `#4F46E5` |
| `color.status.pending` | `#D97706` |
| `color.status.offline` | `#94A3B8` |
| `color.status.synced` | `#059669` |

### Light

| Token | Value |
|-------|-------|
| `color.background.canvas` | `#F6F8FC` |
| `color.background.surface` | `#FFFFFF` |
| `color.background.elevated` | `#FFFFFF` |
| `color.background.subtle` | `#EEF2F7` |
| `color.text.primary` | `#0F172A` |
| `color.text.secondary` | `#475569` |
| `color.text.muted` | `#64748B` |
| `color.border.default` | `#E2E8F0` |
| `color.border.strong` | `#CBD5E1` |
| Primary / status | same as dark (verify contrast on light surfaces) |

## Contrast

All text/status/button combinations must meet WCAG AA. Pair color with label/icon — never color alone.

## Spacing

`spacing.1`–`spacing.8` → 4, 8, 12, 16, 24, 32, 48, 64 px

## Radius

`radius.sm` 6px · `radius.md` 10px · `radius.lg` 16px

## Typography

Expressive but operational. Prefer a distinctive sans for UI (e.g. **Plus Jakarta Sans** or **DM Sans**) — not Inter/Roboto/Arial defaults.

| Token | Use |
|-------|-----|
| `typography.title` | Page titles |
| `typography.heading` | Section headers |
| `typography.body` | Body |
| `typography.label` | Form labels |
| `typography.caption` | Meta / timestamps |
| `typography.mono` | IMEI, refs, auth codes |

## Motion

`motion.fast` 120ms · `motion.normal` 200ms · `motion.slow` 320ms  
Respect `prefers-reduced-motion`.

## Elevation

`shadow.surface`, `shadow.elevated` — subtle only; avoid multi-layer glow.

## Breakpoints

`breakpoint.mobile` 640 · `breakpoint.tablet` 768 · `breakpoint.desktop` 1024 · `breakpoint.wide` 1280

## Component rules

Create components when reused, behavioral, or design-system primitives. Prefer composition over dozens of near-identical variants.

### Primitives
Button, IconButton, Input, Select, Textarea, Checkbox, Radio, Switch, Badge, Avatar, Tooltip, Dialog, Drawer, Sheet, Tabs, Table, Pagination, Breadcrumb, Toast, EmptyState, ErrorState, LoadingState, Skeleton

### Business
RepairStatusBadge, PaymentStatusBadge, SyncStatusBadge, JobTimeline, PaymentSummary, CashHandoverPanel, SupplierPartTrace, RepairSummary, DeviceIdentityPanel, RiskAlert, BranchSelector, EmployeeActivitySummary, InventoryMovementTimeline

## Source of truth

`design-tokens/tokens.json` → generated CSS variables (web) and Compose theme (Android).

## Related

- [ux-principles.md](ux-principles.md)
