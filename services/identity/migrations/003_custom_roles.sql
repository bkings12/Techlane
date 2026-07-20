-- Custom roles + permission catalog (tenant-scoped roles)
CREATE TABLE IF NOT EXISTS identity.permission_catalog (
  code TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT 'general',
  is_system BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS identity.roles (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES identity.tenants(id),
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  is_system BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID,
  UNIQUE (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS identity.role_permissions (
  role_id UUID NOT NULL REFERENCES identity.roles(id) ON DELETE CASCADE,
  permission_code TEXT NOT NULL REFERENCES identity.permission_catalog(code),
  PRIMARY KEY (role_id, permission_code)
);

CREATE INDEX IF NOT EXISTS idx_roles_tenant ON identity.roles (tenant_id);
