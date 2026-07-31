-- Durable pickup code printed on the intake slip / QR. The customer presents
-- this at the counter (or we scan the QR) to release the device — payment still
-- gates the actual handover.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS pickup_code TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_repair_jobs_pickup_code
  ON repair.repair_jobs (tenant_id, pickup_code)
  WHERE pickup_code IS NOT NULL AND deleted_at IS NULL;
