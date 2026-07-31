-- Quick sale: a line item that isn't in the catalog (staff sourced it from a
-- supplier on the spot). variant_id becomes optional; description/unit_price
-- carry the ad-hoc item, unit_cost/supplier_id are internal-only bookkeeping
-- for what we owe the supplier and the margin — never selected by the
-- customer-facing receipt query.
ALTER TABLE sales.sale_items
  ALTER COLUMN variant_id DROP NOT NULL,
  ADD COLUMN IF NOT EXISTS description TEXT,
  ADD COLUMN IF NOT EXISTS unit_cost NUMERIC(12,2),
  ADD COLUMN IF NOT EXISTS supplier_id UUID REFERENCES inventory.suppliers(id);

ALTER TABLE sales.sale_items
  DROP CONSTRAINT IF EXISTS sale_items_variant_or_description;

ALTER TABLE sales.sale_items
  ADD CONSTRAINT sale_items_variant_or_description
  CHECK (variant_id IS NOT NULL OR description IS NOT NULL);
