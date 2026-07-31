-- Distinguish full diagnostic repairs from known, short replacement jobs.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS service_type TEXT NOT NULL DEFAULT 'repair';

ALTER TABLE repair.repair_jobs
  DROP CONSTRAINT IF EXISTS repair_jobs_service_type_check;

ALTER TABLE repair.repair_jobs
  ADD CONSTRAINT repair_jobs_service_type_check
  CHECK (service_type IN ('repair', 'quick_replacement'));

CREATE INDEX IF NOT EXISTS idx_repair_jobs_service_type
  ON repair.repair_jobs (tenant_id, service_type, status);
