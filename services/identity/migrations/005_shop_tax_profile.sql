CREATE TABLE IF NOT EXISTS identity.shop_profiles (
  tenant_id UUID PRIMARY KEY REFERENCES identity.tenants(id),
  legal_name TEXT,
  tin TEXT,
  address_line1 TEXT,
  address_line2 TEXT,
  city TEXT,
  country TEXT NOT NULL DEFAULT 'KE',
  vat_rate_bps INT NOT NULL DEFAULT 1600,
  vat_inclusive BOOLEAN NOT NULL DEFAULT true,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
