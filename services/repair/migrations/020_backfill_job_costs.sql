-- Parts collected before job_costs existed never booked a cost row, so margin
-- still reads as pure labour. Backfill from collected supplier issues, then
-- recompute each job's rolled-up parts_cost from the ledger.

INSERT INTO repair.job_costs
  (id, tenant_id, repair_job_id, cost_type, description, quantity, unit_cost, amount,
   reference_type, reference_id, created_at, created_by)
SELECT
  gen_random_uuid(),
  si.tenant_id,
  si.repair_job_id,
  'part_supplier',
  COALESCE(NULLIF(TRIM(pr.description), ''), 'Part'),
  1,
  si.unit_cost,
  si.unit_cost,
  'supplier_issue',
  si.id,
  COALESCE(si.collected_at, now()),
  si.collected_by
FROM inventory.supplier_issues si
JOIN inventory.part_requests pr ON pr.id = si.part_request_id
WHERE si.status = 'collected'
  AND si.repair_job_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM repair.job_costs c
    WHERE c.tenant_id = si.tenant_id
      AND c.reference_type = 'supplier_issue'
      AND c.reference_id = si.id
  );

-- Keep the denormalised roll-up honest for every job that has cost rows.
UPDATE repair.repair_jobs rj
SET parts_cost = COALESCE((
  SELECT SUM(c.amount) FROM repair.job_costs c
  WHERE c.tenant_id = rj.tenant_id AND c.repair_job_id = rj.id
), 0)
WHERE EXISTS (
  SELECT 1 FROM repair.job_costs c
  WHERE c.tenant_id = rj.tenant_id AND c.repair_job_id = rj.id
)
OR COALESCE(rj.parts_cost, 0) <> 0;
