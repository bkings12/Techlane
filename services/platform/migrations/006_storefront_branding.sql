-- Electro-style storefront branding: shop name, logo, theme colors, topbar
-- links, header promo, and homepage rail visibility toggles. All owner-editable.

ALTER TABLE platform.storefront_settings
  ADD COLUMN IF NOT EXISTS shop_display_name TEXT,
  ADD COLUMN IF NOT EXISTS page_title TEXT,
  ADD COLUMN IF NOT EXISTS color_primary TEXT,
  ADD COLUMN IF NOT EXISTS color_secondary TEXT,
  ADD COLUMN IF NOT EXISTS color_accent TEXT,
  ADD COLUMN IF NOT EXISTS topbar_help_href TEXT,
  ADD COLUMN IF NOT EXISTS topbar_support_href TEXT,
  ADD COLUMN IF NOT EXISTS topbar_contact_href TEXT,
  ADD COLUMN IF NOT EXISTS topbar_phone_label TEXT,
  ADD COLUMN IF NOT EXISTS header_promo_text TEXT,
  ADD COLUMN IF NOT EXISTS show_featured BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS show_new_arrivals BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS show_bestsellers BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS show_deals BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS show_most_viewed BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS logo_object_key TEXT,
  ADD COLUMN IF NOT EXISTS logo_bytes BYTEA,
  ADD COLUMN IF NOT EXISTS logo_content_type TEXT,
  ADD COLUMN IF NOT EXISTS logo_updated_at TIMESTAMPTZ;
