#!/usr/bin/env bash
# Minimal staging smoke: health/ready + login + create repair.
set -euo pipefail

BASE="${PUBLIC_API_BASE:-http://localhost:8080}/api/v1"
EMAIL="${SMOKE_EMAIL:-owner@techlane.local}"
PASSWORD="${SMOKE_PASSWORD:-password}"

echo "GET $BASE/health"
curl -fsS "$BASE/health" | grep -q ok

echo "GET $BASE/ready"
curl -fsS "$BASE/ready" | grep -q ready

echo "POST /auth/login"
TOKEN="$(curl -fsS -X POST "$BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')"

echo "POST /customers"
CUST="$(curl -fsS -X POST "$BASE/customers" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"full_name":"Smoke Customer","phone":"+254700000099"}')"
CUST_ID="$(echo "$CUST" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')"

echo "POST /devices"
DEV="$(curl -fsS -X POST "$BASE/devices" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"customer_id\":\"$CUST_ID\",\"brand\":\"Smoke\",\"model\":\"Phone\",\"imei\":\"SMOKE$(date +%s)\"}")"
DEV_ID="$(echo "$DEV" | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')"

BRANCH="$(curl -fsS "$BASE/branches" -H "Authorization: Bearer $TOKEN" | python3 -c 'import sys,json; print(json.load(sys.stdin)[0]["id"])')"

echo "POST /repairs"
curl -fsS -X POST "$BASE/repairs" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"customer_id\":\"$CUST_ID\",\"device_id\":\"$DEV_ID\",\"branch_id\":\"$BRANCH\",\"problem_summary\":\"Staging smoke test\"}" \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); assert d.get("id"); print("repair", d["id"])'

echo "Staging smoke OK"
