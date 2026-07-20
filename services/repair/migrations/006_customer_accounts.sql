-- Optional storefront accounts reuse repair customers as the customer identity.
ALTER TABLE repair.customers
  ADD COLUMN IF NOT EXISTS password_hash TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_tenant_email_unique
  ON repair.customers(tenant_id, lower(email))
  WHERE email IS NOT NULL;

CREATE TABLE IF NOT EXISTS repair.customer_sessions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  customer_id UUID NOT NULL REFERENCES repair.customers(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_sessions_lookup
  ON repair.customer_sessions(token_hash, expires_at);
