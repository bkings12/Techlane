-- Object storage keys for Cloudflare R2 / MinIO. Content may be null when stored remotely.
ALTER TABLE repair.repair_attachments
  ADD COLUMN IF NOT EXISTS storage_key TEXT;

ALTER TABLE repair.repair_attachments
  ALTER COLUMN content DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_repair_attachments_storage_key
  ON repair.repair_attachments(storage_key)
  WHERE storage_key IS NOT NULL;
