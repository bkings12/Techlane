-- Structured device-condition chips captured at intake (back cover missing, etc.).
ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS condition_tags TEXT[] NOT NULL DEFAULT '{}';
