#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

export APP_ENV="${APP_ENV:-development}"
export DATABASE_URL="${DATABASE_URL:-postgres://techlane:techlane@localhost:5433/techlane?sslmode=disable}"
export JWT_SECRET="${JWT_SECRET:-dev-change-me-in-production-32chars}"
export HTTP_ADDR="${HTTP_ADDR:-:8080}"
export CORS_ORIGINS="${CORS_ORIGINS:-*}"

if ! command -v go >/dev/null; then
  echo "go is required"
  exit 1
fi

go build -o bin/platform ./cmd/platform
exec ./bin/platform
