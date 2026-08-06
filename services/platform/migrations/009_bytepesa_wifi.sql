-- Guest WiFi (BytePesa partner) settings per TechLane tenant
CREATE TABLE IF NOT EXISTS platform.bytepesa_wifi_settings (
  tenant_id uuid PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT false,
  api_base_url text NOT NULL DEFAULT 'https://api.bytepesa.co.ke',
  api_key text NOT NULL DEFAULT '',
  site_id uuid,
  package_id uuid,
  default_duration_mins integer NOT NULL DEFAULT 60,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS platform.bytepesa_wifi_vouchers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  code text NOT NULL,
  redeem_url text NOT NULL,
  qr_payload text NOT NULL,
  duration_mins integer NOT NULL,
  expires_at timestamptz,
  package_name text,
  repair_id uuid,
  sale_id uuid,
  reference text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bytepesa_wifi_vouchers_tenant_idx
  ON platform.bytepesa_wifi_vouchers (tenant_id, created_at DESC);
