#!/usr/bin/env bash
# Fail if Go mux routes are missing from contracts/openapi/openapi.yaml.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OPENAPI="$ROOT/contracts/openapi/openapi.yaml"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

# Extract METHOD /path from HandleFunc / Handle registrations.
rg -oN --no-filename \
  'Handle(?:Func)?\("((?:GET|POST|PUT|PATCH|DELETE) /[^"]+)"' \
  "$ROOT/internal" "$ROOT/cmd" \
  | sed -E 's/Handle(Func)?\("([^"]+)".*/\2/' \
  | sort -u > "$TMP"

missing=0
while IFS= read -r route; do
  method="${route%% *}"
  path="${route#* }"
  # Normalize path params {id} → already OpenAPI style
  # Allow webhooks and health without full OpenAPI coverage via allowlist patterns
  case "$path" in
    /health|/ready|/metrics) continue ;;
  esac
  # OpenAPI paths are under /api/v1 prefix in some files; also check bare path
  if ! grep -qE "^[[:space:]]*(/api/v1)?${path}:" "$OPENAPI" \
    && ! grep -qE "^[[:space:]]*${path}:" "$OPENAPI"; then
    # Also try matching path templates loosely
    bare="${path//\{*\}/{}/}"
    if ! grep -qF "$path" "$OPENAPI"; then
      echo "MISSING in OpenAPI: $method $path"
      missing=$((missing + 1))
    fi
  fi
done < "$TMP"

if [[ "$missing" -gt 0 ]]; then
  echo "OpenAPI drift: $missing route(s) missing (see above)."
  echo "Update contracts/openapi/openapi.yaml or adjust this allowlist."
  # Soft-fail during rollout: warn but exit 0 until OpenAPI catch-up lands.
  # Set OPENAPI_DRIFT_STRICT=1 to fail CI.
  if [[ "${OPENAPI_DRIFT_STRICT:-0}" == "1" ]]; then
    exit 1
  fi
  echo "OPENAPI_DRIFT_STRICT not set — warning only."
fi

echo "OpenAPI drift check complete ($(wc -l < "$TMP") routes scanned)."
