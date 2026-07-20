#!/usr/bin/env bash
# Restore a custom-format dump into TARGET_DATABASE_URL (defaults to DATABASE_URL).
set -euo pipefail

DUMP_FILE="${1:?usage: restore-postgres.sh <dump-file>}"
TARGET_DATABASE_URL="${TARGET_DATABASE_URL:-${DATABASE_URL:?DATABASE_URL or TARGET_DATABASE_URL required}}"

if [[ ! -f "$DUMP_FILE" ]]; then
  echo "dump not found: $DUMP_FILE" >&2
  exit 1
fi

echo "Restoring $DUMP_FILE into $TARGET_DATABASE_URL"
pg_restore --clean --if-exists --no-owner --no-acl --dbname="$TARGET_DATABASE_URL" "$DUMP_FILE"
echo "Restore complete"
psql "$TARGET_DATABASE_URL" -c "SELECT current_database(), now();"
