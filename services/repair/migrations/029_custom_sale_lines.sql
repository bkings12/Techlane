-- Allow ad-hoc charges (diagnosis fee, one-off items) without inventory.
ALTER TABLE repair.job_sale_lines
  ALTER COLUMN variant_id DROP NOT NULL,
  ALTER COLUMN location_id DROP NOT NULL,
  ADD COLUMN IF NOT EXISTS is_custom BOOLEAN NOT NULL DEFAULT false;
