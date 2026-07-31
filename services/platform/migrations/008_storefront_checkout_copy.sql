-- Owner-editable checkout payment labels / hints / CTAs (no hardcoded storefront copy).
ALTER TABLE platform.storefront_settings
  ADD COLUMN IF NOT EXISTS pay_label_stk TEXT,
  ADD COLUMN IF NOT EXISTS pay_label_paybill TEXT,
  ADD COLUMN IF NOT EXISTS pay_label_cash TEXT,
  ADD COLUMN IF NOT EXISTS pay_hint_stk TEXT,
  ADD COLUMN IF NOT EXISTS pay_hint_paybill TEXT,
  ADD COLUMN IF NOT EXISTS pay_hint_cash TEXT,
  ADD COLUMN IF NOT EXISTS pay_cta_stk TEXT,
  ADD COLUMN IF NOT EXISTS pay_cta_paybill TEXT,
  ADD COLUMN IF NOT EXISTS pay_cta_cash TEXT;
