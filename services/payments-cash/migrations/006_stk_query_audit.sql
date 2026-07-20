ALTER TABLE payments.mpesa_stk_transactions
  ADD COLUMN IF NOT EXISTS last_queried_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS raw_callback JSONB,
  ADD COLUMN IF NOT EXISTS query_result_code TEXT,
  ADD COLUMN IF NOT EXISTS query_result_desc TEXT;

CREATE TABLE IF NOT EXISTS payments.store_credits (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  customer_id UUID NOT NULL,
  balance NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
  currency TEXT NOT NULL DEFAULT 'KES',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, customer_id)
);

CREATE TABLE IF NOT EXISTS payments.store_credit_ledger (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  customer_id UUID NOT NULL,
  amount NUMERIC(14,2) NOT NULL,
  entry_type TEXT NOT NULL,
  payment_id UUID,
  note TEXT,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  correlation_id UUID
);

CREATE INDEX IF NOT EXISTS store_credit_ledger_customer_idx
  ON payments.store_credit_ledger (tenant_id, customer_id, created_at DESC);
