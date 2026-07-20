-- Presigned direct-to-R2 uploads (pending until client calls complete).
ALTER TABLE repair.repair_attachments
  ADD COLUMN IF NOT EXISTS upload_status TEXT NOT NULL DEFAULT 'completed';

ALTER TABLE repair.repair_attachments
  ADD COLUMN IF NOT EXISTS sha256_hex TEXT;

UPDATE repair.repair_attachments
SET upload_status = 'completed'
WHERE upload_status IS NULL OR upload_status = '';

CREATE INDEX IF NOT EXISTS idx_repair_attachments_pending_upload
  ON repair.repair_attachments(tenant_id, repair_job_id, created_at DESC)
  WHERE upload_status = 'pending';
