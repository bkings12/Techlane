-- payments schema
CREATE TABLE IF NOT EXISTS payments.payments (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  method TEXT NOT NULL,
  amount NUMERIC(12,2) NOT NULL,
  currency TEXT NOT NULL DEFAULT 'KES',
  status TEXT NOT NULL,
  received_by UUID,
  drawer_id UUID,
  provider_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID,
  version INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS payments.payment_allocations (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  payment_id UUID NOT NULL REFERENCES payments.payments(id),
  payable_type TEXT NOT NULL,
  payable_id UUID NOT NULL,
  amount NUMERIC(12,2) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments.cash_drawers (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments.cash_ledger_entries (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  drawer_id UUID,
  employee_id UUID NOT NULL,
  amount NUMERIC(12,2) NOT NULL,
  entry_type TEXT NOT NULL,
  payment_id UUID REFERENCES payments.payments(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS payments.cash_handovers (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID NOT NULL,
  from_user_id UUID NOT NULL,
  to_user_id UUID,
  amount NUMERIC(12,2) NOT NULL,
  status TEXT NOT NULL,
  shortage_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
  confirmed_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  confirmed_at TIMESTAMPTZ,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS payments.mpesa_transactions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  payment_id UUID REFERENCES payments.payments(id),
  checkout_request_id TEXT,
  merchant_request_id TEXT,
  result_code TEXT,
  raw_payload JSONB,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments.refunds (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  payment_id UUID NOT NULL REFERENCES payments.payments(id),
  amount NUMERIC(12,2) NOT NULL,
  status TEXT NOT NULL,
  reason TEXT,
  approved_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID
);

CREATE TABLE IF NOT EXISTS payments.idempotency_records (
  action_id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  body_hash TEXT NOT NULL,
  status_code INT,
  response JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
