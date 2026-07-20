-- Labor amount used as commission accrual base when a repair is completed
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS labor_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
