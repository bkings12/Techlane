-- Hierarchical inventory categories for POS catalog browsing.
ALTER TABLE inventory.categories
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

DO $$ BEGIN
  ALTER TABLE inventory.categories
    ADD CONSTRAINT categories_parent_fk
    FOREIGN KEY (parent_id) REFERENCES inventory.categories(id) ON DELETE RESTRICT;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  ALTER TABLE inventory.products
    ADD CONSTRAINT products_category_fk
    FOREIGN KEY (category_id) REFERENCES inventory.categories(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Sibling names unique under the same parent (NULL parent = root).
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_tenant_parent_name
  ON inventory.categories (
    tenant_id,
    (COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid)),
    lower(name)
  );

CREATE INDEX IF NOT EXISTS idx_categories_tenant_parent
  ON inventory.categories (tenant_id, parent_id);

CREATE INDEX IF NOT EXISTS idx_products_tenant_category_id
  ON inventory.products (tenant_id, category_id);

-- Promote freeform product.category strings into root categories, then link.
INSERT INTO inventory.categories (id, tenant_id, name, parent_id)
SELECT gen_random_uuid(), src.tenant_id, src.category, NULL
FROM (
  SELECT DISTINCT tenant_id, btrim(category) AS category
  FROM inventory.products
  WHERE category IS NOT NULL AND btrim(category) <> ''
) src
WHERE NOT EXISTS (
  SELECT 1 FROM inventory.categories c
  WHERE c.tenant_id = src.tenant_id
    AND c.parent_id IS NULL
    AND lower(c.name) = lower(src.category)
);

UPDATE inventory.products pr
SET category_id = c.id
FROM inventory.categories c
WHERE pr.category_id IS NULL
  AND pr.category IS NOT NULL
  AND btrim(pr.category) <> ''
  AND c.tenant_id = pr.tenant_id
  AND c.parent_id IS NULL
  AND lower(c.name) = lower(btrim(pr.category));
