-- Generic work-order line items (labour/part/product) so a repair can price and
-- report each separately while still billing the customer one combined total.
-- Additive: job_sale_lines/job_costs/labor_amount keep working for existing jobs.
CREATE TABLE IF NOT EXISTS repair.job_line_items (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs (id) ON DELETE CASCADE,
  line_type TEXT NOT NULL CHECK (line_type IN ('labour', 'part', 'product')),
  description TEXT NOT NULL,
  quantity NUMERIC(12, 2) NOT NULL DEFAULT 1 CHECK (quantity > 0),
  unit_price NUMERIC(12, 2) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
  unit_cost NUMERIC(12, 2),
  discount_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
  line_total NUMERIC(12, 2) NOT NULL,
  variant_id UUID,
  location_id UUID,
  part_source TEXT CHECK (part_source IN ('inventory', 'sourced')),
  part_status TEXT CHECK (part_status IN ('required', 'sourcing', 'ordered', 'received', 'installed', 'returned', 'cancelled')),
  supplier_name TEXT,
  supplier_ref TEXT,
  expected_arrival DATE,
  added_to_inventory_at TIMESTAMPTZ,
  reference_type TEXT,
  reference_id UUID,
  created_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_job_line_items_job
  ON repair.job_line_items (tenant_id, repair_job_id);
CREATE INDEX IF NOT EXISTS idx_job_line_items_type
  ON repair.job_line_items (tenant_id, repair_job_id, line_type);
