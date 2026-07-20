ALTER TABLE identity.registered_devices
  ADD COLUMN IF NOT EXISTS fingerprint TEXT,
  ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_registered_devices_tenant_user
  ON identity.registered_devices (tenant_id, user_id)
  WHERE revoked_at IS NULL;
