# TechLane WhatsApp sidecar

Baileys-based WhatsApp Web sessions, one per TechLane **tenant UUID**.
The platform API proxies QR/status/send and receives inbound replies at
`POST /api/v1/whatsapp/inbound`.

## Run locally

```bash
cd services/whatsapp-service
cp ../../.env.example .env.local   # or export vars
npm ci
WHATSAPP_SERVICE_SECRET=dev-secret \
WHATSAPP_INBOUND_URL=http://127.0.0.1:8080/api/v1/whatsapp/inbound \
WHATSAPP_PORT=3001 \
npm start
```

Platform `.env`:

```
WHATSAPP_SERVICE_URL=http://127.0.0.1:3001
WHATSAPP_SERVICE_SECRET=dev-secret
```

Owner dashboard: **Settings → WhatsApp** — enable, scan QR (WhatsApp or WhatsApp Business → Linked devices), toggle customers/suppliers.

## Replies

| Who | Message | Action |
|-----|---------|--------|
| Customer | `YES` / `NO` | Approve / reject pending estimate |
| Supplier | `QUOTE 2500` / `DECLINE` | Submit / decline part quote |

Do not expose port 3001 publicly; only the platform should reach it.
