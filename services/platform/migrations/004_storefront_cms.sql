-- Storefront CMS: everything the public web-storefront renders is owner-set
-- from web-ops, not hardcoded. Deals carry a real deal_price resolved server
-- side at checkout (see internal/commerce.StartCheckout) — never a client-
-- supplied discount.

CREATE TABLE IF NOT EXISTS platform.storefront_settings (
  tenant_id UUID PRIMARY KEY,

  hero_headline TEXT,
  hero_subtext TEXT,
  hero_cta_label TEXT,
  hero_cta_href TEXT,

  newsletter_headline TEXT,
  newsletter_subtext TEXT,

  footer_tagline TEXT,
  social_facebook TEXT,
  social_instagram TEXT,
  social_twitter TEXT,
  social_tiktok TEXT,
  contact_phone TEXT,
  contact_email TEXT,
  business_hours TEXT,
  app_store_url TEXT,
  play_store_url TEXT,

  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID
);

CREATE TABLE IF NOT EXISTS platform.storefront_banners (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,

  headline TEXT,
  subtext TEXT,
  cta_label TEXT,
  cta_href TEXT,

  image_object_key TEXT,
  image_bytes BYTEA,
  image_content_type TEXT,
  image_updated_at TIMESTAMPTZ,

  sort_order INT NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_storefront_banners_tenant_active
  ON platform.storefront_banners (tenant_id, active, sort_order);

CREATE TABLE IF NOT EXISTS platform.storefront_deals (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  variant_id UUID NOT NULL REFERENCES inventory.product_variants(id) ON DELETE CASCADE,

  title TEXT,
  deal_price NUMERIC(12,2) NOT NULL CHECK (deal_price > 0),
  ends_at TIMESTAMPTZ,
  active BOOLEAN NOT NULL DEFAULT true,
  sort_order INT NOT NULL DEFAULT 0,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The exact lookup StartCheckout and ListOnlineCatalog do on every request.
CREATE INDEX IF NOT EXISTS idx_storefront_deals_active_lookup
  ON platform.storefront_deals (tenant_id, variant_id)
  WHERE active;

CREATE TABLE IF NOT EXISTS platform.newsletter_subscribers (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  email TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_newsletter_subscribers_tenant_email
  ON platform.newsletter_subscribers (tenant_id, lower(email));

-- Preserves the pre-discount price for "was/now" display on order lookups,
-- even if the deal is later edited or removed.
ALTER TABLE sales.order_items ADD COLUMN IF NOT EXISTS original_unit_price NUMERIC(12,2);

-- Storefront merchandising flags — read by the public catalog/home sections
-- only; POS/repair never reference these.
ALTER TABLE inventory.products
  ADD COLUMN IF NOT EXISTS featured BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS new_arrival BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS bestseller BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS storefront_sort_order INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_products_tenant_featured ON inventory.products (tenant_id) WHERE featured;
CREATE INDEX IF NOT EXISTS idx_products_tenant_new_arrival ON inventory.products (tenant_id) WHERE new_arrival;
CREATE INDEX IF NOT EXISTS idx_products_tenant_bestseller ON inventory.products (tenant_id) WHERE bestseller;
