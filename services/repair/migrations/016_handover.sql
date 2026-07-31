-- Handover verification at collection.
--
-- Until now a job reached 'collected' as a side effect of the balance being paid,
-- which is not the same event: money clearing does not mean the device left the
-- counter. This records who actually took the device and how we established they
-- were entitled to it, so a disputed handover has an answer.

CREATE TABLE IF NOT EXISTS repair.job_handovers (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs(id) ON DELETE CASCADE,
  collected_by_name TEXT NOT NULL,
  -- 'self' when the owner collects, otherwise who they are to the owner.
  relationship TEXT NOT NULL DEFAULT 'self',
  id_number TEXT,
  phone TEXT,
  -- 'otp': the owner's phone confirmed a code at the counter.
  -- 'staff_vouched': a manager released it without a code and owns that decision.
  verification_method TEXT NOT NULL CHECK (verification_method IN ('otp', 'staff_vouched')),
  verified_at TIMESTAMPTZ,
  released_by UUID,
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One handover per job: a device is handed over once.
CREATE UNIQUE INDEX IF NOT EXISTS idx_job_handovers_job
  ON repair.job_handovers (tenant_id, repair_job_id);

-- Handover OTP codes are scoped to a job, not just a phone, so a code sent for
-- one collection cannot release a different device.
ALTER TABLE repair.customer_otp_challenges
  ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'login';
ALTER TABLE repair.customer_otp_challenges
  ADD COLUMN IF NOT EXISTS repair_job_id UUID;

CREATE INDEX IF NOT EXISTS idx_customer_otp_challenges_handover
  ON repair.customer_otp_challenges (tenant_id, repair_job_id, created_at DESC)
  WHERE repair_job_id IS NOT NULL;

ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS collected_at TIMESTAMPTZ;

-- Existing collected jobs were handed over before this was tracked; stamp the
-- time we know about so the column is not misleadingly empty.
UPDATE repair.repair_jobs
SET collected_at = updated_at
WHERE status = 'collected' AND collected_at IS NULL;
