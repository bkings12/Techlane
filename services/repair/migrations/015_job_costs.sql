-- Job cost ledger. Until now a repair's parts cost lived only on supplier issues,
-- so a part taken off the shop's own shelf was invisible: stock never moved and the
-- job looked pure profit. Every part consumed on a job now books a cost row here,
-- whichever way it was sourced, which is what makes per-job margin real.
CREATE TABLE IF NOT EXISTS repair.job_costs (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL,
  cost_type TEXT NOT NULL,
  description TEXT NOT NULL,
  quantity INT NOT NULL DEFAULT 1,
  unit_cost NUMERIC(12, 2) NOT NULL DEFAULT 0,
  amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
  reference_type TEXT,
  reference_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_job_costs_job
  ON repair.job_costs (tenant_id, repair_job_id);

-- One cost row per source document, so a retried collection or a double-tapped
-- "issue from stock" cannot inflate the cost of a job.
CREATE UNIQUE INDEX IF NOT EXISTS uq_job_costs_reference
  ON repair.job_costs (tenant_id, reference_type, reference_id)
  WHERE reference_id IS NOT NULL;

-- Parts issued from own stock need somewhere to record which variant and location
-- they came from, so the stock movement and the job cost reconcile.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS parts_cost NUMERIC(12, 2) NOT NULL DEFAULT 0;
