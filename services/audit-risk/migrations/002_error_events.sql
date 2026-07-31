-- Self-hosted "errors inbox": captures unhandled panics and 5xx responses so
-- there's somewhere to see failures besides stdout logs (see docs/observability-plan.md).
CREATE TABLE IF NOT EXISTS audit.error_events (
  id UUID PRIMARY KEY,
  tenant_id UUID,
  method TEXT NOT NULL,
  route TEXT NOT NULL,
  status INT NOT NULL,
  message TEXT NOT NULL,
  stack TEXT,
  correlation_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_error_events_time ON audit.error_events (created_at DESC);
