-- Recoverable deletion for intake mistakes and duplicate job cards.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deleted_by UUID;

CREATE INDEX IF NOT EXISTS idx_repair_jobs_trash
  ON repair.repair_jobs (tenant_id, deleted_at DESC)
  WHERE deleted_at IS NOT NULL;
