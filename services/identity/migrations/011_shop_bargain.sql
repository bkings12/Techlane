-- Storefront bargain-via-WhatsApp: owner toggle + public contact number.
ALTER TABLE identity.shop_profiles
  ADD COLUMN IF NOT EXISTS bargain_enabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS whatsapp_number TEXT;
