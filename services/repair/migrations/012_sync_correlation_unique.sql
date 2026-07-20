-- Idempotent offline sync: one domain row per correlation_id (action_id) per tenant.
CREATE UNIQUE INDEX IF NOT EXISTS uq_repair_jobs_tenant_correlation
  ON repair.repair_jobs (tenant_id, correlation_id)
  WHERE correlation_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_repair_notes_tenant_correlation
  ON repair.repair_notes (tenant_id, correlation_id)
  WHERE correlation_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_repair_attachments_tenant_correlation
  ON repair.repair_attachments (tenant_id, correlation_id)
  WHERE correlation_id IS NOT NULL;
