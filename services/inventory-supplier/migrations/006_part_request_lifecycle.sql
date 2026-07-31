-- A part request on a job that gets cancelled or written off must be voided:
-- an 'approved' request is a live authorisation to draw a part from a supplier's
-- shelf on the shop's account, and leaving it open after the job dies is a leak.
--
-- note explains why a credit entry exists. Reversing a supplier's balance without
-- a recorded reason makes the ledger unauditable, which defeats its purpose.
ALTER TABLE inventory.supplier_credit_entries
  ADD COLUMN IF NOT EXISTS note TEXT;

-- Chasing "what is this job still waiting for?" is the hottest read in the parts
-- flow: it runs on every collection and on every job detail load.
CREATE INDEX IF NOT EXISTS idx_part_requests_job_open
  ON inventory.part_requests (tenant_id, repair_job_id)
  WHERE status IN ('pending', 'approved');
