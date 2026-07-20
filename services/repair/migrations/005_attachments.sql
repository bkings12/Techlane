-- Photos and documents captured during repair intake and diagnosis.
CREATE TABLE IF NOT EXISTS repair.repair_attachments (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs(id) ON DELETE CASCADE,
  file_name TEXT NOT NULL,
  content_type TEXT NOT NULL,
  content BYTEA NOT NULL,
  size_bytes INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE INDEX IF NOT EXISTS idx_repair_attachments_job
  ON repair.repair_attachments(tenant_id, repair_job_id, created_at DESC);
