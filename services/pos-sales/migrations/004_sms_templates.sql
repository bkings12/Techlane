-- Editable SMS message templates (per tenant). Empty/missing rows use built-in defaults.
CREATE TABLE IF NOT EXISTS notify.sms_templates (
  tenant_id UUID NOT NULL,
  template_key TEXT NOT NULL,
  body TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID,
  PRIMARY KEY (tenant_id, template_key)
);

CREATE INDEX IF NOT EXISTS idx_sms_templates_tenant ON notify.sms_templates(tenant_id);
