# Android Information Architecture

## App

`apps/android` — Kotlin + Jetpack Compose APK for technicians and cashiers.

## Navigation

Bottom nav (role-aware):

| Tab | Technician | Cashier |
|-----|------------|---------|
| Home | Job queue | Sale / pay home |
| Jobs | All assigned | Repair pay lookup |
| Scan | Barcode/QR/IMEI | Same |
| More | Parts, sync, profile | Cash, sync, profile |

## Screens (MVP)

1. Login / branch select
2. Home (role dashboard cards — few, actionable)
3. Job list (filters: mine, waiting parts, ready)
4. Job detail (status, notes, request part, photos)
5. Repair intake wizard (stepper)
6. Part request / auth code display
7. Part collection confirm (scan auth code)
8. Payment: cash provisional / trigger STK (online)
9. Cash drawer & handover request
10. Sync center (pending/conflicts)
11. Offline banner + pending count

## Mobile patterns

- Lists and cards, not data tables
- Bottom sheets for actions
- FAB for primary create where appropriate
- CameraX for photos; ML Kit for barcodes
- Large primary buttons; thumb-zone actions

## Offline UX

- Global offline chip
- Per-item sync badges
- Sync center for retries and conflicts
- Block sensitive actions with clear “requires connection” state

## Shared with web

Tokens, status vocabulary (`pending`, `confirmed`, `synced`), job terminology — different layout density and interaction.
