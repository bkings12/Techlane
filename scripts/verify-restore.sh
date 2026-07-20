#!/usr/bin/env bash
# Restore into a scratch database and run basic smoke SQL.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DUMP_FILE="${1:?usage: verify-restore.sh <dump-file>}"
ADMIN_URL="${ADMIN_DATABASE_URL:-${DATABASE_URL:?ADMIN_DATABASE_URL or DATABASE_URL required}}"
SCRATCH_DB="${SCRATCH_DB:-techlane_restore_verify}"

psql "$ADMIN_URL" -v ON_ERROR_STOP=1 <<SQL
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${SCRATCH_DB}' AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS ${SCRATCH_DB};
CREATE DATABASE ${SCRATCH_DB};
SQL

# Point at scratch DB (replace final path segment)
SCRATCH_URL="$(python3 - <<PY
import os, urllib.parse
u = urllib.parse.urlparse(os.environ["ADMIN_URL"])
path = "/${SCRATCH_DB}"
print(urllib.parse.urlunparse(u._replace(path=path)))
PY
)"
export ADMIN_URL
export SCRATCH_DB

TARGET_DATABASE_URL="$SCRATCH_URL" "$ROOT/scripts/restore-postgres.sh" "$DUMP_FILE"

psql "$SCRATCH_URL" -v ON_ERROR_STOP=1 <<'SQL'
SELECT 1 AS ok;
SELECT COUNT(*) AS tenants FROM identity.tenants;
SELECT COUNT(*) AS repairs FROM repair.repair_jobs;
SQL

echo "Restore verification OK"
