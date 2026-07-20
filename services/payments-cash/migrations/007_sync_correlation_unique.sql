CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_tenant_correlation
  ON payments.payments (tenant_id, correlation_id)
  WHERE correlation_id IS NOT NULL;
