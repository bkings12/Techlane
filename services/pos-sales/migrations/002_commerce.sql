-- commerce readiness extensions (Phase 5 scaffolding)
CREATE TABLE IF NOT EXISTS inventory.carts (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  customer_id UUID,
  channel TEXT NOT NULL DEFAULT 'online',
  status TEXT NOT NULL DEFAULT 'open',
  branch_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.cart_items (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  cart_id UUID NOT NULL REFERENCES inventory.carts(id),
  variant_id UUID NOT NULL,
  quantity INT NOT NULL,
  unit_price NUMERIC(12,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS sales.orders (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  customer_id UUID,
  channel TEXT NOT NULL DEFAULT 'online',
  status TEXT NOT NULL,
  collection_code TEXT,
  fulfilment_type TEXT NOT NULL DEFAULT 'branch_pickup',
  subtotal NUMERIC(12,2) NOT NULL DEFAULT 0,
  tax_total NUMERIC(12,2) NOT NULL DEFAULT 0,
  discount_total NUMERIC(12,2) NOT NULL DEFAULT 0,
  delivery_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
  total NUMERIC(12,2) NOT NULL DEFAULT 0,
  reservation_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  correlation_id UUID,
  version INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS sales.order_items (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  order_id UUID NOT NULL REFERENCES sales.orders(id),
  variant_id UUID NOT NULL,
  quantity INT NOT NULL,
  unit_price NUMERIC(12,2) NOT NULL,
  line_total NUMERIC(12,2) NOT NULL,
  fulfilment_status TEXT NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_orders_collection_code ON sales.orders(tenant_id, collection_code);
