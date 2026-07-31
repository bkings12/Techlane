-- Loyalty points groundwork: per-tenant settings, a running balance per
-- customer, and an append-only ledger so balances are always reconstructible
-- and auditable. Accrual sources are generic (reference_type/reference_id)
-- so new triggers (POS sales, referrals, manual adjustments) can be added
-- without a schema change.

CREATE TABLE IF NOT EXISTS loyalty.settings (
  tenant_id UUID PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT false,
  points_per_completed_repair INT NOT NULL DEFAULT 10,
  points_per_currency_unit NUMERIC(10, 4) NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS loyalty.accounts (
  tenant_id UUID NOT NULL,
  customer_id UUID NOT NULL,
  points_balance INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, customer_id)
);

CREATE TABLE IF NOT EXISTS loyalty.ledger (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  customer_id UUID NOT NULL,
  delta INT NOT NULL,
  reason TEXT NOT NULL,
  reference_type TEXT,
  reference_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_loyalty_ledger_customer ON loyalty.ledger (tenant_id, customer_id, created_at DESC);

-- Outbound marketing/integration webhooks: tenants register a URL + the
-- event types they care about (e.g. "repair.completed", "payment.confirmed")
-- and we fire a signed POST whenever a matching event passes through the bus.
CREATE TABLE IF NOT EXISTS loyalty.webhook_subscriptions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  url TEXT NOT NULL,
  secret TEXT NOT NULL,
  event_types TEXT[] NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  last_triggered_at TIMESTAMPTZ,
  last_status TEXT
);

CREATE INDEX IF NOT EXISTS idx_loyalty_webhooks_tenant ON loyalty.webhook_subscriptions (tenant_id) WHERE is_active;
