-- Storefront engagement: verified-purchase product reviews, product view
-- counts for "Most Viewed", a cached FX-rate table for the display-only
-- currency switcher, and supporting columns on banners/settings.

CREATE TABLE IF NOT EXISTS platform.product_reviews (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
  customer_id UUID NOT NULL REFERENCES repair.customers(id) ON DELETE CASCADE,
  order_id UUID NOT NULL REFERENCES sales.orders(id),

  rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  title TEXT,
  body TEXT,
  status TEXT NOT NULL DEFAULT 'published',

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One review per customer per product; resubmitting updates it in place.
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_reviews_one_per_customer
  ON platform.product_reviews (tenant_id, customer_id, product_id);

CREATE INDEX IF NOT EXISTS idx_product_reviews_product_published
  ON platform.product_reviews (tenant_id, product_id)
  WHERE status = 'published';

CREATE TABLE IF NOT EXISTS platform.product_view_counts (
  tenant_id UUID NOT NULL,
  variant_id UUID NOT NULL,
  view_count BIGINT NOT NULL DEFAULT 0,
  last_viewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, variant_id)
);

CREATE TABLE IF NOT EXISTS platform.fx_rate_cache (
  base_code TEXT PRIMARY KEY,
  rates JSONB NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL
);

-- Which slide a banner renders as, and an optional link to a real deal so a
-- hero/promo tile can show an actual was/now price instead of pure copy.
ALTER TABLE platform.storefront_banners
  ADD COLUMN IF NOT EXISTS placement TEXT NOT NULL DEFAULT 'hero',
  ADD COLUMN IF NOT EXISTS deal_id UUID REFERENCES platform.storefront_deals(id) ON DELETE SET NULL;

-- Display-only currency switcher (checkout always settles in KES via M-Pesa)
-- and four owner-editable trust badges — no hardcoded "Worldwide Shipping"
-- claims for a branch-pickup business.
ALTER TABLE platform.storefront_settings
  ADD COLUMN IF NOT EXISTS enabled_currencies TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_1_title TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_1_subtext TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_2_title TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_2_subtext TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_3_title TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_3_subtext TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_4_title TEXT,
  ADD COLUMN IF NOT EXISTS trust_badge_4_subtext TEXT;
