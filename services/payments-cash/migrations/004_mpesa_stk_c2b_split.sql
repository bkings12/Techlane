-- STK and C2B use separate ledgers (different Safaricom payloads and lifecycle).
CREATE TABLE IF NOT EXISTS payments.mpesa_stk_transactions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  payment_id UUID NOT NULL REFERENCES payments.payments(id),
  phone TEXT,
  account_reference TEXT,
  checkout_request_id TEXT,
  merchant_request_id TEXT,
  mpesa_receipt TEXT,
  result_code TEXT,
  result_desc TEXT,
  raw_payload JSONB,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (payment_id)
);

CREATE TABLE IF NOT EXISTS payments.mpesa_c2b_transactions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  payment_id UUID REFERENCES payments.payments(id),
  trans_id TEXT,
  trans_type TEXT,
  trans_time TEXT,
  amount NUMERIC(12,2),
  business_shortcode TEXT,
  bill_ref_number TEXT,
  invoice_number TEXT,
  msisdn TEXT,
  first_name TEXT,
  middle_name TEXT,
  last_name TEXT,
  org_account_balance TEXT,
  third_party_trans_id TEXT,
  raw_payload JSONB,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments.bank_transactions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  payment_id UUID NOT NULL REFERENCES payments.payments(id),
  paybill TEXT,
  account_number TEXT,
  provider_ref TEXT,
  raw_payload JSONB,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (payment_id)
);

CREATE INDEX IF NOT EXISTS idx_mpesa_stk_checkout
  ON payments.mpesa_stk_transactions (tenant_id, checkout_request_id)
  WHERE checkout_request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_mpesa_c2b_trans_id
  ON payments.mpesa_c2b_transactions (tenant_id, trans_id)
  WHERE trans_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_mpesa_c2b_bill_ref
  ON payments.mpesa_c2b_transactions (tenant_id, bill_ref_number)
  WHERE bill_ref_number IS NOT NULL;

-- Move rows out of the legacy shared table when present.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'payments' AND table_name = 'mpesa_transactions'
  ) THEN
    INSERT INTO payments.mpesa_stk_transactions (
      id, tenant_id, payment_id, phone, account_reference,
      checkout_request_id, merchant_request_id, result_code, raw_payload, status, created_at, updated_at
    )
    SELECT m.id, m.tenant_id, m.payment_id, m.phone, m.account_reference,
           m.checkout_request_id, m.merchant_request_id, m.result_code, m.raw_payload, m.status, m.created_at, m.updated_at
    FROM payments.mpesa_transactions m
    JOIN payments.payments p ON p.id = m.payment_id
    WHERE p.method = 'mpesa_stk'
      AND NOT EXISTS (SELECT 1 FROM payments.mpesa_stk_transactions s WHERE s.payment_id = m.payment_id)
    ON CONFLICT (payment_id) DO NOTHING;

    INSERT INTO payments.mpesa_c2b_transactions (
      id, tenant_id, payment_id, msisdn, bill_ref_number, raw_payload, status, created_at, updated_at
    )
    SELECT m.id, m.tenant_id, m.payment_id, m.phone, m.account_reference, m.raw_payload, m.status, m.created_at, m.updated_at
    FROM payments.mpesa_transactions m
    JOIN payments.payments p ON p.id = m.payment_id
    WHERE p.method = 'mpesa_c2b'
      AND NOT EXISTS (SELECT 1 FROM payments.mpesa_c2b_transactions c WHERE c.payment_id = m.payment_id);

    INSERT INTO payments.bank_transactions (
      id, tenant_id, payment_id, account_number, status, created_at, updated_at
    )
    SELECT m.id, m.tenant_id, m.payment_id, m.account_reference, m.status, m.created_at, m.updated_at
    FROM payments.mpesa_transactions m
    JOIN payments.payments p ON p.id = m.payment_id
    WHERE p.method IN ('bank_paybill', 'bank_transfer')
      AND NOT EXISTS (SELECT 1 FROM payments.bank_transactions b WHERE b.payment_id = m.payment_id)
    ON CONFLICT (payment_id) DO NOTHING;
  END IF;
END $$;
