-- Tenant-editable suggestion catalogs for intake (condition chips + common issues).
-- Labels are copied onto repair_jobs as plain strings; changing a preset never rewrites history.
CREATE TABLE IF NOT EXISTS repair.intake_presets (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('condition_tag', 'issue')),
  label TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  is_system BOOLEAN NOT NULL DEFAULT false,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, kind, label)
);

CREATE INDEX IF NOT EXISTS intake_presets_tenant_kind_idx
  ON repair.intake_presets (tenant_id, kind, sort_order, label);
