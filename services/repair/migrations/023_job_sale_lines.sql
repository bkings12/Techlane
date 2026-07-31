-- Counter accessories sold with a repair (cases, chargers, glass, etc.).
-- These add to the customer balance on the same payable so one cash/STK
-- payment can cover repair + extras.
CREATE TABLE IF NOT EXISTS repair.job_sale_lines (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs (id) ON DELETE CASCADE,
    variant_id UUID NOT NULL,
    location_id UUID NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(14, 2) NOT NULL CHECK (unit_price >= 0),
    line_total NUMERIC(14, 2) NOT NULL CHECK (line_total >= 0),
    unit_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_job_sale_lines_job
    ON repair.job_sale_lines (tenant_id, repair_job_id);
