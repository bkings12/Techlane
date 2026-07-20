-- Supplier portal identity, quotes, and assignment on part requests.

CREATE TABLE IF NOT EXISTS inventory.supplier_contacts (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  supplier_id UUID NOT NULL REFERENCES inventory.suppliers(id),
  email TEXT NOT NULL,
  phone TEXT,
  display_name TEXT NOT NULL,
  password_hash TEXT,
  status TEXT NOT NULL CHECK (status IN ('invited', 'active', 'revoked')),
  invite_token_hash TEXT,
  invite_expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, email)
);

CREATE INDEX IF NOT EXISTS idx_supplier_contacts_invite
  ON inventory.supplier_contacts(invite_token_hash)
  WHERE invite_token_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_supplier_contacts_supplier
  ON inventory.supplier_contacts(tenant_id, supplier_id);

CREATE TABLE IF NOT EXISTS inventory.supplier_sessions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  contact_id UUID NOT NULL REFERENCES inventory.supplier_contacts(id),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_supplier_sessions_contact
  ON inventory.supplier_sessions(tenant_id, contact_id);

CREATE TABLE IF NOT EXISTS inventory.part_request_quotes (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  part_request_id UUID NOT NULL REFERENCES inventory.part_requests(id),
  supplier_id UUID NOT NULL REFERENCES inventory.suppliers(id),
  unit_cost NUMERIC(12,2) NOT NULL,
  notes TEXT,
  status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'declined', 'superseded')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_part_request_quotes_request
  ON inventory.part_request_quotes(tenant_id, part_request_id);

CREATE INDEX IF NOT EXISTS idx_part_request_quotes_supplier
  ON inventory.part_request_quotes(tenant_id, supplier_id, status);

ALTER TABLE inventory.part_requests
  ADD COLUMN IF NOT EXISTS assigned_supplier_id UUID REFERENCES inventory.suppliers(id),
  ADD COLUMN IF NOT EXISTS quote_status TEXT;

CREATE INDEX IF NOT EXISTS idx_part_requests_assigned_supplier
  ON inventory.part_requests(tenant_id, assigned_supplier_id)
  WHERE assigned_supplier_id IS NOT NULL;
