CREATE TABLE IF NOT EXISTS platform.receipt_settings (
  tenant_id UUID PRIMARY KEY,

  header_note TEXT,
  phone TEXT,
  email TEXT,
  website TEXT,
  show_logo BOOLEAN NOT NULL DEFAULT true,
  show_address BOOLEAN NOT NULL DEFAULT true,
  show_tin BOOLEAN NOT NULL DEFAULT true,

  thank_you_text TEXT,
  footer_text TEXT,
  warranty_text TEXT,

  show_vat_breakdown BOOLEAN NOT NULL DEFAULT true,
  show_imei BOOLEAN NOT NULL DEFAULT true,
  show_payments BOOLEAN NOT NULL DEFAULT true,
  show_balance BOOLEAN NOT NULL DEFAULT true,
  show_served_by BOOLEAN NOT NULL DEFAULT true,

  default_paper TEXT NOT NULL DEFAULT 'thermal80',

  number_prefix TEXT NOT NULL DEFAULT 'RCT-',
  next_number BIGINT NOT NULL DEFAULT 1,

  logo_object_key TEXT,
  logo_content_type TEXT,
  logo_bytes BYTEA,
  logo_updated_at TIMESTAMPTZ,

  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by UUID
);

-- A receipt number is allocated once per document so reprints keep the same
-- serial. Without this a customer could hold two receipts for one job.
CREATE TABLE IF NOT EXISTS platform.receipt_numbers (
  tenant_id UUID NOT NULL,
  doc_type TEXT NOT NULL,
  doc_id UUID NOT NULL,
  number BIGINT NOT NULL,
  formatted TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, doc_type, doc_id)
);

CREATE INDEX IF NOT EXISTS receipt_numbers_tenant_created_idx
  ON platform.receipt_numbers (tenant_id, created_at DESC);
