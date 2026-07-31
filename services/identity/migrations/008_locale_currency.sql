-- i18n / multi-currency groundwork: every tenant picks a display currency and
-- locale once; the frontend formats all money/date values through these
-- instead of hardcoding "KES" / en-KE everywhere.
ALTER TABLE identity.shop_profiles
  ADD COLUMN IF NOT EXISTS currency_code TEXT NOT NULL DEFAULT 'KES',
  ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT 'en-KE';
