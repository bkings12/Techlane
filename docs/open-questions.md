# Open Questions

Resolved defaults are listed first. Remaining questions need business input only when they change architecture.

## Resolved (technical defaults)

| Topic | Decision |
|-------|----------|
| Frontend | React + Vite |
| Android | Native Kotlin Compose |
| Backend | Go microservices monorepo |
| megamart | Reference only; not merged |
| DB layout | Multi-schema single Postgres |
| Multi-tenant | `tenant_id` now; single business deploy |
| Commerce | Schema-ready; storefront Phase 5 |
| Sync service | In gateway+domains for MVP |

## Pending (non-blocking for Phase 0–2)

1. **Primary SMS/WhatsApp provider** for customer notifications (Africa's Talking, Twilio, etc.)  
2. **Production object storage** — Cloudflare R2 vs AWS S3 vs other  
3. **Legal entity / tax** display rules for receipts (VAT inclusive vs exclusive)  
4. **Supplier auth code delivery** — show in app only vs SMS to collector  
5. **Customer portal auth** — magic link vs PIN vs OTP  
6. **Brand name lock** for customer-facing storefront later  

## Explicitly not open

- Whether employees can type M-Pesa refs to mark paid → **No**  
- Whether cash handover can be self-approved → **No**  
- Whether parts can leave without reason → **No**  
