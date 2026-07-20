ALTER TABLE payments.mpesa_transactions
  ADD COLUMN IF NOT EXISTS phone TEXT,
  ADD COLUMN IF NOT EXISTS account_reference TEXT;

CREATE INDEX IF NOT EXISTS idx_mpesa_checkout
  ON payments.mpesa_transactions (tenant_id, checkout_request_id)
  WHERE checkout_request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cash_handovers_open
  ON payments.cash_handovers (tenant_id, status, created_at DESC);
