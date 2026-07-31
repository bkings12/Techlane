-- Closure states: a job can now end as 'cancelled' or 'unrepairable' instead of
-- being left open forever or faked as 'completed' (which would wrongly accrue
-- commission, award loyalty points and issue a warranty).
--
-- closure_reason holds a structured code (see internal/repair/status.go) so the
-- "why do we lose jobs?" question is a GROUP BY rather than a text search. The
-- operator's free text stays in repair_status_events.note alongside it.
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS closure_reason TEXT,
  ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_repair_jobs_closure
  ON repair.repair_jobs (tenant_id, closure_reason)
  WHERE closure_reason IS NOT NULL;
