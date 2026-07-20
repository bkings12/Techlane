-- Basic merchandising fields shared by POS and the public storefront.
ALTER TABLE inventory.products
  ADD COLUMN IF NOT EXISTS category TEXT,
  ADD COLUMN IF NOT EXISTS description TEXT,
  ADD COLUMN IF NOT EXISTS image_url TEXT;

CREATE INDEX IF NOT EXISTS idx_products_tenant_category
  ON inventory.products(tenant_id, category);
