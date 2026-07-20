-- Tenant SMS provider credentials (BlessedTexts for customer OTP).
CREATE TABLE IF NOT EXISTS repair.sms_settings (
  tenant_id UUID PRIMARY KEY,
  provider TEXT NOT NULL DEFAULT 'blessedtexts',
  enabled BOOLEAN NOT NULL DEFAULT false,
  api_key TEXT NOT NULL DEFAULT '',
  sender_id TEXT NOT NULL DEFAULT '',
  base_url TEXT NOT NULL DEFAULT 'https://sms.blessedtexts.com/api/sms/v1',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID
);
