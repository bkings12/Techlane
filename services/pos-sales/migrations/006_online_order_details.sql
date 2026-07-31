-- Guest checkout details + delivery address for online storefront orders.
ALTER TABLE sales.orders
	ADD COLUMN IF NOT EXISTS guest_name TEXT,
	ADD COLUMN IF NOT EXISTS guest_phone TEXT,
	ADD COLUMN IF NOT EXISTS guest_email TEXT,
	ADD COLUMN IF NOT EXISTS customer_notes TEXT,
	ADD COLUMN IF NOT EXISTS delivery_address_line1 TEXT,
	ADD COLUMN IF NOT EXISTS delivery_address_line2 TEXT,
	ADD COLUMN IF NOT EXISTS delivery_city TEXT,
	ADD COLUMN IF NOT EXISTS delivery_landmark TEXT;

CREATE INDEX IF NOT EXISTS idx_orders_guest_phone
	ON sales.orders (tenant_id, guest_phone)
	WHERE guest_phone IS NOT NULL AND guest_phone <> '';
