# Production activation checklist (single shop)

Use this after staging smoke passes. Code and scripts live in-repo; live credentials are supplied at deploy time.

## Infrastructure

- [ ] DNS for `API_DOMAIN`, `OPS_DOMAIN`, `PORTAL_DOMAIN`, `SUPPLIER_DOMAIN`
- [ ] TLS via Caddy (`deploy/Caddyfile` + `deploy/docker-compose.prod.yml`)
- [ ] `deploy/.env.prod` from `.env.prod.example` (never commit secrets)
- [ ] `JWT_SECRET` ≥ 32 chars, non-default
- [ ] `CORS_ORIGINS` explicit https allowlist (not `*`)
- [ ] `APP_ENV=production`
- [ ] Postgres backups: cron `scripts/backup-postgres.sh`; restore drill with `scripts/verify-restore.sh`
- [ ] Uptime check on `https://$API_DOMAIN/ready`

## Object storage (Cloudflare R2)

- [ ] Private R2 bucket + API token
- [ ] `OBJECT_STORAGE_*` set (see `deploy/object-storage.env.example`)
- [ ] Run `go run ./cmd/migrate-attachments --dry-run` then migrate BYTEA rows
- [ ] Verify photo upload (presign or base64) and download on a real repair

## SMS (BlessedTexts)

- [ ] API key + sender ID in Settings → SMS **or** `BLESSEDTEXTS_*` env
- [ ] Send test OTP to a real phone; confirm portal/customer login

## M-Pesa (Safaricom Daraja)

- [ ] Production shortcode, consumer key/secret, passkey in Settings → Payments
- [ ] `PUBLIC_API_BASE` + optional `MPESA_WEBHOOK_TOKEN`
- [ ] Callback URL https and registered with Safaricom (STK + C2B)
- [ ] Sandbox STK end-to-end on staging; then production STK + Query reconcile
- [ ] Confirm staff “Reconcile” requires owner/manager (no typed receipt as sole proof)

## Shop / tax

- [ ] Settings → Shop profile: legal name, TIN, address, VAT rate (bps), VAT inclusive
- [ ] Print sample receipt HTML + PDF; tax invoice PDF

## Android release

- [ ] Release `API_BASE` points at production API
- [ ] Signing keystore configured (see `apps/android/README.md`)
- [ ] Internal track / sideload staff, customer, supplier APKs
- [ ] Device register after login; revoke test in web-ops

## Go-live smoke

```bash
export PUBLIC_API_BASE=https://api.example.com
export SMOKE_EMAIL=owner@…
export SMOKE_PASSWORD=…
./scripts/smoke-staging.sh
```

- [ ] Create repair → part request → payment → receipt PDF
- [ ] Offline draft → note → sync (staff app)
- [ ] Supplier quote → issue → voucher PDF
- [ ] Customer OTP → pay STK → warranty visible after complete
