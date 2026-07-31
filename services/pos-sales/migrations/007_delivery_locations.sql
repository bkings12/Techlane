-- Priced delivery areas the shop owner configures for storefront checkout.
CREATE TABLE IF NOT EXISTS sales.delivery_locations (
	id UUID PRIMARY KEY,
	tenant_id UUID NOT NULL,
	name TEXT NOT NULL,
	description TEXT,
	fee NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (fee >= 0),
	active BOOLEAN NOT NULL DEFAULT TRUE,
	sort_order INT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_delivery_locations_tenant
	ON sales.delivery_locations (tenant_id, active, sort_order, name);

ALTER TABLE sales.orders
	ADD COLUMN IF NOT EXISTS delivery_location_id UUID;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = 'orders_delivery_location_id_fkey'
	) THEN
		ALTER TABLE sales.orders DROP CONSTRAINT orders_delivery_location_id_fkey;
	END IF;
	ALTER TABLE sales.orders
		ADD CONSTRAINT orders_delivery_location_id_fkey
		FOREIGN KEY (delivery_location_id) REFERENCES sales.delivery_locations(id)
		ON DELETE SET NULL;
END $$;
