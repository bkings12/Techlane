#!/usr/bin/env bash
# Nightly-friendly Postgres backup. Uploads to R2/S3 when OBJECT_STORAGE_* is set.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="${BACKUP_DIR:-$ROOT/backups}"
mkdir -p "$OUT_DIR"

DATABASE_URL="${DATABASE_URL:?DATABASE_URL is required}"
FILE="$OUT_DIR/techlane-$TIMESTAMP.dump"

echo "Backing up to $FILE"
pg_dump --format=custom --no-owner --no-acl "$DATABASE_URL" --file="$FILE"
echo "Backup size: $(du -h "$FILE" | awk '{print $1}')"

# Optional upload via AWS CLI compatible with Cloudflare R2
if [[ -n "${OBJECT_STORAGE_BUCKET:-}" && -n "${OBJECT_STORAGE_ENDPOINT:-}" ]]; then
  KEY="backups/postgres/techlane-$TIMESTAMP.dump"
  echo "Uploading to s3://${OBJECT_STORAGE_BUCKET}/${KEY}"
  AWS_ACCESS_KEY_ID="${OBJECT_STORAGE_ACCESS_KEY:?}" \
  AWS_SECRET_ACCESS_KEY="${OBJECT_STORAGE_SECRET_KEY:?}" \
  aws --endpoint-url "$OBJECT_STORAGE_ENDPOINT" s3 cp "$FILE" "s3://${OBJECT_STORAGE_BUCKET}/${KEY}"
fi

# Retention: keep last N local dumps
KEEP="${BACKUP_KEEP:-14}"
ls -1t "$OUT_DIR"/techlane-*.dump 2>/dev/null | tail -n +"$((KEEP + 1))" | xargs -r rm -f
echo "Done. Local retention keep=$KEEP"
