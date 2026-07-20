-- Human-readable sequential job codes per tenant (JOB-101, JOB-102, …)
CREATE TABLE IF NOT EXISTS repair.job_counters (
  tenant_id UUID PRIMARY KEY,
  next_number INT NOT NULL
);

ALTER TABLE repair.repair_jobs
  ADD COLUMN IF NOT EXISTS job_number INT,
  ADD COLUMN IF NOT EXISTS job_code TEXT;

WITH numbered AS (
  SELECT id,
         100 + ROW_NUMBER() OVER (PARTITION BY tenant_id ORDER BY created_at, id) AS n
  FROM repair.repair_jobs
  WHERE job_number IS NULL
)
UPDATE repair.repair_jobs j
SET job_number = numbered.n,
    job_code = 'JOB-' || numbered.n::text
FROM numbered
WHERE j.id = numbered.id;

INSERT INTO repair.job_counters (tenant_id, next_number)
SELECT tenant_id, COALESCE(MAX(job_number), 100) + 1
FROM repair.repair_jobs
GROUP BY tenant_id
ON CONFLICT (tenant_id) DO UPDATE
SET next_number = GREATEST(repair.job_counters.next_number, EXCLUDED.next_number);

-- Seed counters for tenants that have no jobs yet (noop-safe if identity tenants exist elsewhere)
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'identity' AND table_name = 'tenants') THEN
    INSERT INTO repair.job_counters (tenant_id, next_number)
    SELECT id, 101 FROM identity.tenants
    ON CONFLICT (tenant_id) DO NOTHING;
  END IF;
END $$;

UPDATE repair.repair_jobs
SET job_code = 'JOB-' || job_number::text
WHERE job_code IS NULL AND job_number IS NOT NULL;

ALTER TABLE repair.repair_jobs
  ALTER COLUMN job_number SET NOT NULL,
  ALTER COLUMN job_code SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_repair_jobs_tenant_job_number
  ON repair.repair_jobs (tenant_id, job_number);
CREATE UNIQUE INDEX IF NOT EXISTS uq_repair_jobs_tenant_job_code
  ON repair.repair_jobs (tenant_id, job_code);
CREATE INDEX IF NOT EXISTS idx_repair_jobs_job_code
  ON repair.repair_jobs (tenant_id, job_code);

-- Rewrite orphan alert titles that still embed raw UUIDs
UPDATE audit.risk_alerts a
SET title = 'Orphan part collected for ' || j.job_code,
    details = COALESCE(a.details, '{}'::jsonb) || jsonb_build_object('job_code', j.job_code, 'repair_job_id', j.id::text)
FROM inventory.supplier_issues si
JOIN repair.repair_jobs j ON j.id = si.repair_job_id
WHERE a.kind = 'orphan_part'
  AND a.entity_type = 'supplier_issue'
  AND a.entity_id = si.id;
