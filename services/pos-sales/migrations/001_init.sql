-- sales + gateway + notify
CREATE TABLE IF NOT EXISTS sales.sales (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID NOT NULL,
  customer_id UUID,
  channel TEXT NOT NULL DEFAULT 'pos',
  status TEXT NOT NULL,
  subtotal NUMERIC(12,2) NOT NULL DEFAULT 0,
  tax_total NUMERIC(12,2) NOT NULL DEFAULT 0,
  discount_total NUMERIC(12,2) NOT NULL DEFAULT 0,
  total NUMERIC(12,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID,
  version INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS sales.sale_items (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  sale_id UUID NOT NULL REFERENCES sales.sales(id),
  variant_id UUID NOT NULL,
  quantity INT NOT NULL,
  unit_price NUMERIC(12,2) NOT NULL,
  line_total NUMERIC(12,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS gateway.sync_commands (
  action_id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  device_id UUID,
  user_id UUID NOT NULL,
  command_type TEXT NOT NULL,
  local_timestamp TIMESTAMPTZ,
  payload JSONB NOT NULL,
  body_hash TEXT NOT NULL,
  sync_status TEXT NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  last_error TEXT,
  result JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notify.notification_outbox (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  channel TEXT NOT NULL,
  recipient TEXT NOT NULL,
  template_key TEXT,
  payload JSONB,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at TIMESTAMPTZ
);
