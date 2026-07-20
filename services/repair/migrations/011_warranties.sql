CREATE TABLE IF NOT EXISTS repair.warranties (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs(id),
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  duration_days INT NOT NULL DEFAULT 90,
  status TEXT NOT NULL DEFAULT 'active',
  claim_note TEXT,
  claimed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, repair_job_id)
);

CREATE INDEX IF NOT EXISTS idx_warranties_tenant_repair
  ON repair.warranties (tenant_id, repair_job_id);
