-- Per-tenant WhatsApp (Baileys sidecar) preferences. Session auth lives on the sidecar;
-- TechLane only stores whether the shop wants to use WhatsApp for customers/suppliers.
CREATE TABLE IF NOT EXISTS platform.whatsapp_settings (
  tenant_id UUID PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT false,
  notify_customers BOOLEAN NOT NULL DEFAULT true,
  notify_suppliers BOOLEAN NOT NULL DEFAULT true,
  also_send_sms BOOLEAN NOT NULL DEFAULT false,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID
);

-- Correlate inbound YES / QUOTE replies to the last outbound ask.
CREATE TABLE IF NOT EXISTS platform.whatsapp_pending_actions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  phone_digits TEXT NOT NULL,
  action_type TEXT NOT NULL,
  ref_id UUID NOT NULL,
  repair_job_id UUID,
  job_code TEXT,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_pending_phone
  ON platform.whatsapp_pending_actions (tenant_id, phone_digits, expires_at DESC);
