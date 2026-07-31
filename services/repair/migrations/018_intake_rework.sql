-- Structured intake evidence and warranty rework lineage.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS intake_accessories JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS intake_condition TEXT,
  ADD COLUMN IF NOT EXISTS device_passcode_ciphertext BYTEA,
  ADD COLUMN IF NOT EXISTS parent_job_id UUID REFERENCES repair.repair_jobs(id),
  ADD COLUMN IF NOT EXISTS rework_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_repair_jobs_parent
  ON repair.repair_jobs (tenant_id, parent_job_id)
  WHERE parent_job_id IS NOT NULL;
