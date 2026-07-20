-- C2B TransID must be unique per tenant for webhook idempotency.
CREATE UNIQUE INDEX IF NOT EXISTS idx_mpesa_c2b_trans_id_unique
  ON payments.mpesa_c2b_transactions (tenant_id, trans_id)
  WHERE trans_id IS NOT NULL AND trans_id <> '';

CREATE INDEX IF NOT EXISTS idx_mpesa_c2b_unmatched
  ON payments.mpesa_c2b_transactions (tenant_id, status, created_at DESC)
  WHERE status IN ('unmatched', 'amount_mismatch');
