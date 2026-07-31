-- Store-locator contact details, shown on the public web-storefront's
-- branch list. All optional — an empty field just doesn't render there.
ALTER TABLE identity.branches
  ADD COLUMN IF NOT EXISTS phone TEXT,
  ADD COLUMN IF NOT EXISTS hours TEXT,
  ADD COLUMN IF NOT EXISTS map_url TEXT;
