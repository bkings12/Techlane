-- Normalized phone uniqueness (digits only) for OTP login lookups.
CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_tenant_phone_normalized
  ON repair.customers (tenant_id, (regexp_replace(phone, '[^0-9]', '', 'g')))
  WHERE phone IS NOT NULL AND regexp_replace(phone, '[^0-9]', '', 'g') <> '';

CREATE TABLE IF NOT EXISTS repair.customer_otp_challenges (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  phone_e164 TEXT NOT NULL,
  code_hash TEXT NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_otp_challenges_lookup
  ON repair.customer_otp_challenges (tenant_id, phone_e164, created_at DESC);

CREATE TABLE IF NOT EXISTS repair.repair_estimates (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs(id) ON DELETE CASCADE,
  labor_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
  parts_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'KES',
  notes TEXT,
  status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
  expires_at TIMESTAMPTZ,
  created_by UUID,
  decided_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_repair_estimates_job
  ON repair.repair_estimates (tenant_id, repair_job_id, created_at DESC);
