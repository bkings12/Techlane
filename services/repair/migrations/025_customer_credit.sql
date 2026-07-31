-- Customer credit is an explicit intake decision. It permits handover with an
-- outstanding balance and gives the owner dashboard a durable due date.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS customer_credit BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS credit_due_date DATE,
  ADD COLUMN IF NOT EXISTS credit_reminder_sent_at TIMESTAMPTZ;

ALTER TABLE repair.repair_jobs
  DROP CONSTRAINT IF EXISTS repair_jobs_credit_due_date_check;

ALTER TABLE repair.repair_jobs
  ADD CONSTRAINT repair_jobs_credit_due_date_check
  CHECK (
    (customer_credit = false AND credit_due_date IS NULL)
    OR (customer_credit = true AND credit_due_date IS NOT NULL)
  );

CREATE INDEX IF NOT EXISTS idx_repair_jobs_credit_due
  ON repair.repair_jobs (tenant_id, credit_due_date)
  WHERE customer_credit = true AND status = 'collected';
