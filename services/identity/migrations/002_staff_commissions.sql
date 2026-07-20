-- Staff profiles and optional technician commissions
CREATE TABLE IF NOT EXISTS identity.employee_profiles (
  user_id UUID PRIMARY KEY REFERENCES identity.users(id),
  tenant_id UUID NOT NULL REFERENCES identity.tenants(id),
  employee_code TEXT,
  phone TEXT,
  is_technician BOOLEAN NOT NULL DEFAULT false,
  commission_enabled BOOLEAN NOT NULL DEFAULT false,
  commission_type TEXT NOT NULL DEFAULT 'none',
  percent_bps INT,
  fixed_amount NUMERIC(12,2),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID
);

CREATE TABLE IF NOT EXISTS identity.commission_entries (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  user_id UUID NOT NULL REFERENCES identity.users(id),
  repair_job_id UUID NOT NULL,
  entry_type TEXT NOT NULL,
  base_amount NUMERIC(12,2) NOT NULL,
  commission_amount NUMERIC(12,2) NOT NULL,
  currency TEXT NOT NULL DEFAULT 'KES',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID,
  UNIQUE (tenant_id, repair_job_id, user_id, entry_type)
);

CREATE INDEX IF NOT EXISTS idx_commission_entries_status
  ON identity.commission_entries (tenant_id, status, user_id);
