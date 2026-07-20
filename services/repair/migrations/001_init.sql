-- repair schema
CREATE TABLE IF NOT EXISTS repair.customers (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  full_name TEXT NOT NULL,
  phone TEXT,
  email TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS repair.devices (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  customer_id UUID REFERENCES repair.customers(id),
  anonymous BOOLEAN NOT NULL DEFAULT false,
  kind TEXT NOT NULL,
  brand TEXT,
  model TEXT,
  imei TEXT,
  serial_number TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS repair.repair_jobs (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID NOT NULL,
  customer_id UUID,
  device_id UUID NOT NULL REFERENCES repair.devices(id),
  technician_id UUID,
  status TEXT NOT NULL,
  problem_summary TEXT NOT NULL,
  version INT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  updated_by UUID,
  source_device_id UUID,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS repair.repair_status_events (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  repair_job_id UUID NOT NULL REFERENCES repair.repair_jobs(id),
  status TEXT NOT NULL,
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE INDEX IF NOT EXISTS idx_repair_jobs_tenant_branch ON repair.repair_jobs(tenant_id, branch_id);
CREATE INDEX IF NOT EXISTS idx_repair_jobs_status ON repair.repair_jobs(tenant_id, status);
