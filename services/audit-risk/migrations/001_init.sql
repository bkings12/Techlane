-- audit schema
CREATE TABLE IF NOT EXISTS audit.audit_events (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  actor_id UUID,
  actor_role TEXT,
  device_id UUID,
  ip_address TEXT,
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id UUID,
  previous_value JSONB,
  new_value JSONB,
  reason TEXT,
  approval_ref UUID,
  correlation_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit.risk_alerts (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  kind TEXT NOT NULL,
  severity TEXT NOT NULL,
  title TEXT NOT NULL,
  entity_type TEXT,
  entity_id UUID,
  status TEXT NOT NULL DEFAULT 'open',
  details JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  resolved_by UUID
);

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_time ON audit.audit_events(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_alerts_open ON audit.risk_alerts(tenant_id, status, kind);
