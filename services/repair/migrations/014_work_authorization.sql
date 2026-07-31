-- Work authorisation: a job may not go on the bench until somebody has agreed to
-- a price. Either the customer approved an estimate, or a manager recorded a
-- verbal go-ahead for a walk-in ("just fix it"). Without this, the shop does the
-- work first and argues about money afterwards.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS work_authorized_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS work_authorized_by UUID,
  ADD COLUMN IF NOT EXISTS work_authorization_source TEXT,
  ADD COLUMN IF NOT EXISTS authorized_amount NUMERIC(12, 2),
  ADD COLUMN IF NOT EXISTS labor_variance_reason TEXT;

-- Jobs already past intake predate the gate. Treat them as authorised at their
-- creation time, otherwise every open job on the bench would jam.
UPDATE repair.repair_jobs
SET work_authorized_at = created_at,
    work_authorization_source = 'legacy',
    authorized_amount = labor_amount
WHERE work_authorized_at IS NULL
  AND status IN ('in_progress', 'completed', 'collected');
