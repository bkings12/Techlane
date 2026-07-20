-- Technician diagnosis / work notes on repair jobs
CREATE TABLE IF NOT EXISTS repair.repair_notes (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs(id) ON DELETE CASCADE,
  note TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE INDEX IF NOT EXISTS idx_repair_notes_job ON repair.repair_notes(tenant_id, repair_job_id, created_at);
