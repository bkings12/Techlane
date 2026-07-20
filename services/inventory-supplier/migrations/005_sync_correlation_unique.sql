CREATE UNIQUE INDEX IF NOT EXISTS uq_part_requests_tenant_correlation
  ON inventory.part_requests (tenant_id, correlation_id)
  WHERE correlation_id IS NOT NULL;
