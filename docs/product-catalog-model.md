# Product Catalog Model

## Entities

- **Product** — name, brand, category, description, condition continuum support, online visibility flags, optional `merchant_id`
- **ProductCategory** — tree; defines allowed specification schema
- **ProductBrand**
- **ProductVariant** — SKU, barcode, attributes (color, storage…), prices (sell, cost, wholesale, promo), tax category, warranty template ref, reorder level
- **ProductImage** — R2 keys, sort order
- **Serial/IMEI unit** (optional table) — for serialized retail/repair devices

## Specification model

Do **not** put phone-specific columns on `products`.

`product_variants.specs` JSONB validated against category JSON Schema:

**Phone:** storage, RAM, color, SIM type, battery capacity  
**Laptop:** processor, RAM, storage, screen size, OS  
**Accessory:** compatibility, connector type, color, brand  

## Pricing

Backend owns calculation:

- Base / branch / channel / customer-group / promo (later)
- UI displays; server validates payable amount

## Condition

Enum or attribute: `new`, `used`, `refurbished` — not separate product tables.

## Channel visibility

`pos_visible`, `online_visible`, `online_published_at`

## Search (future)

Postgres full-text sufficient initially. Introduce Meilisearch/Typesense/OpenSearch when catalog size or UX requires faceted speed — documented trigger, not Phase 0.

## Ownership

Inventory-supplier service owns catalog tables until a dedicated product service is justified.
