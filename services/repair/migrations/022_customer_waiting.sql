-- While-you-wait / wait-bench intake: customer stays at the shop instead of
-- leaving the device for later collection. estimated_wait_minutes is the verbal
-- commitment staff gives at the counter; SMS uses the same figure.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS customer_waiting BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS estimated_wait_minutes INT;

CREATE INDEX IF NOT EXISTS idx_repair_jobs_customer_waiting
  ON repair.repair_jobs (tenant_id, branch_id)
  WHERE customer_waiting AND deleted_at IS NULL
    AND status NOT IN ('ready_for_pickup', 'completed', 'collected', 'cancelled', 'unrepairable');
