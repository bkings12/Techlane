-- Catalog list price at time of sale (for bargained / overridden unit_price).
ALTER TABLE sales.sale_items
  ADD COLUMN IF NOT EXISTS list_price NUMERIC(14,2);
