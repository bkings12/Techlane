-- Tenant payment provider credentials (Daraja / M-Pesa).
-- Bank paybill reuses the same M-Pesa API credentials; only paybill + account are extra.
CREATE TABLE IF NOT EXISTS payments.provider_settings (
  tenant_id UUID PRIMARY KEY,
  environment TEXT NOT NULL DEFAULT 'sandbox',
  mpesa_enabled BOOLEAN NOT NULL DEFAULT false,
  mpesa_shortcode TEXT NOT NULL DEFAULT '',
  mpesa_consumer_key TEXT NOT NULL DEFAULT '',
  mpesa_consumer_secret TEXT NOT NULL DEFAULT '',
  mpesa_passkey TEXT NOT NULL DEFAULT '',
  mpesa_callback_url TEXT NOT NULL DEFAULT '',
  bank_enabled BOOLEAN NOT NULL DEFAULT false,
  bank_paybill TEXT NOT NULL DEFAULT '',
  bank_account TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID
);
