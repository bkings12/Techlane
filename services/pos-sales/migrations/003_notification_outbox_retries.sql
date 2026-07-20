-- Extend notification outbox for retries and staff inbox
ALTER TABLE notify.notification_outbox
  ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error TEXT,
  ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_notification_outbox_pending
  ON notify.notification_outbox (status, next_attempt_at, created_at)
  WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS notify.staff_inbox (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  branch_id UUID,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  template_key TEXT,
  payload JSONB,
  read_at TIMESTAMPTZ,
  acked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_staff_inbox_tenant_unacked
  ON notify.staff_inbox (tenant_id, created_at DESC)
  WHERE acked_at IS NULL;
