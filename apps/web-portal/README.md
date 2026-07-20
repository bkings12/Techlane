# TechLane customer repair portal

OTP-first customer web app for tracking repairs, approving estimates, and paying balances. Guest job-code lookup remains available as a secondary path.

```bash
cd apps/web-portal
npm install
npm run dev
```

Dev server: `http://localhost:5175`  
API default: `http://localhost:8080/api/v1` (override with `VITE_API_BASE`)

## Flows
- **Signed in:** phone OTP → repair list → detail (estimate approve/reject, M-Pesa STK, receipts)
- **Guest:** job code + phone → read-only public status

OTP SMS must be configured by an owner under web-ops **Settings → SMS (OTP)**.
