-- inventory schema (commerce-ready)
CREATE TABLE IF NOT EXISTS inventory.suppliers (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  phone TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.categories (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  parent_id UUID,
  spec_schema JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS inventory.products (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  category_id UUID,
  brand TEXT,
  name TEXT NOT NULL,
  condition TEXT NOT NULL DEFAULT 'new',
  pos_visible BOOLEAN NOT NULL DEFAULT true,
  online_visible BOOLEAN NOT NULL DEFAULT false,
  merchant_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.product_variants (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  product_id UUID NOT NULL REFERENCES inventory.products(id),
  sku TEXT NOT NULL,
  barcode TEXT,
  specs JSONB NOT NULL DEFAULT '{}'::jsonb,
  sell_price NUMERIC(12,2) NOT NULL DEFAULT 0,
  cost_price NUMERIC(12,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, sku)
);

CREATE TABLE IF NOT EXISTS inventory.stock_locations (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  name TEXT NOT NULL,
  location_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.inventory_balances (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  variant_id UUID NOT NULL REFERENCES inventory.product_variants(id),
  location_id UUID NOT NULL REFERENCES inventory.stock_locations(id),
  physical_qty INT NOT NULL DEFAULT 0,
  available_qty INT NOT NULL DEFAULT 0,
  reserved_qty INT NOT NULL DEFAULT 0,
  in_transit_qty INT NOT NULL DEFAULT 0,
  damaged_qty INT NOT NULL DEFAULT 0,
  version INT NOT NULL DEFAULT 1,
  UNIQUE (variant_id, location_id)
);

CREATE TABLE IF NOT EXISTS inventory.inventory_movements (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  variant_id UUID NOT NULL,
  location_id UUID NOT NULL,
  qty_delta INT NOT NULL,
  reason TEXT NOT NULL,
  reference_type TEXT,
  reference_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS inventory.inventory_reservations (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  variant_id UUID NOT NULL,
  location_id UUID NOT NULL,
  quantity INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  expires_at TIMESTAMPTZ NOT NULL,
  reference_type TEXT,
  reference_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.part_requests (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID NOT NULL,
  repair_job_id UUID NOT NULL,
  variant_id UUID,
  description TEXT NOT NULL,
  quantity INT NOT NULL DEFAULT 1,
  status TEXT NOT NULL,
  requested_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS inventory.supplier_issues (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID NOT NULL,
  supplier_id UUID NOT NULL REFERENCES inventory.suppliers(id),
  part_request_id UUID NOT NULL REFERENCES inventory.part_requests(id),
  repair_job_id UUID NOT NULL,
  auth_code TEXT NOT NULL,
  unit_cost NUMERIC(12,2) NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  requested_by UUID NOT NULL,
  approved_by UUID NOT NULL,
  collected_by UUID,
  collected_at TIMESTAMPTZ,
  reconciliation_status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  correlation_id UUID,
  UNIQUE (tenant_id, auth_code)
);

CREATE TABLE IF NOT EXISTS inventory.supplier_credit_entries (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  supplier_id UUID NOT NULL,
  supplier_issue_id UUID REFERENCES inventory.supplier_issues(id),
  amount NUMERIC(12,2) NOT NULL,
  entry_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE INDEX IF NOT EXISTS idx_supplier_issues_orphan ON inventory.supplier_issues(tenant_id, status, reconciliation_status);
