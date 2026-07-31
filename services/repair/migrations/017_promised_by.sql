-- Operational commitment and aging support.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS promised_by TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_repair_jobs_overdue
  ON repair.repair_jobs (tenant_id, promised_by)
  WHERE promised_by IS NOT NULL
    AND status NOT IN ('ready_for_pickup', 'completed', 'collected', 'cancelled', 'unrepairable');
