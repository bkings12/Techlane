-- Product images for storefront catalog / deals (same pattern as storefront banners).
ALTER TABLE inventory.products
  ADD COLUMN IF NOT EXISTS image_object_key TEXT,
  ADD COLUMN IF NOT EXISTS image_bytes BYTEA,
  ADD COLUMN IF NOT EXISTS image_content_type TEXT,
  ADD COLUMN IF NOT EXISTS image_updated_at TIMESTAMPTZ;
