#!/usr/bin/env bash
# Promised-by, QC, encrypted intake, and linked rework smoke test.
set -euo pipefail

BASE="${BASE:-http://localhost:8080/api/v1}"
EMAIL="${EMAIL:-owner@techlane.local}"
PASSWORD="${PASSWORD:-password}"
jqr() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval(sys.argv[1]))" "$1"; }
pass() { printf '  ok   %s\n' "$1"; }
fail() { printf '  FAIL %s\n' "$1"; exit 1; }

TOKEN=$(curl -sS -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jqr "d['tokens']['access_token']")
AUTH=(-H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json')
BRANCH=$(curl -sS "${AUTH[@]}" "$BASE/branches" | jqr "d['items'][0]['id']")
CUST=$(curl -sS -X POST "$BASE/customers" "${AUTH[@]}" \
  -d "{\"full_name\":\"Workflow Smoke\",\"phone\":\"0729$RANDOM$RANDOM\"}" | jqr "d['id']")
DEVICE=$(curl -sS -X POST "$BASE/devices" "${AUTH[@]}" \
  -d "{\"customer_id\":\"$CUST\",\"kind\":\"phone\",\"brand\":\"Test\",\"model\":\"Workflow\"}" | jqr "d['id']")
PAST=$(python3 -c "from datetime import datetime,timedelta,timezone;print((datetime.now(timezone.utc)-timedelta(days=1)).isoformat())")

echo "1. structured intake and promised-by"
JOB=$(curl -sS -X POST "$BASE/repairs" "${AUTH[@]}" -d "{
  \"branch_id\":\"$BRANCH\",\"customer_id\":\"$CUST\",\"device_id\":\"$DEVICE\",
  \"problem_summary\":\"workflow smoke\",\"labor_amount\":2000,\"promised_by\":\"$PAST\",
  \"intake_accessories\":[\"Charger\",\"Case\"],\"intake_condition\":\"crack at top left\",
  \"device_passcode\":\"2580\"}" | jqr "d['id']")
DETAIL=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB")
[ "$(printf '%s' "$DETAIL" | jqr "d['has_device_passcode']")" = "True" ] || fail "passcode was not encrypted"
printf '%s' "$DETAIL" | grep -q "Charger" || fail "accessories missing"
pass "intake evidence persisted without exposing passcode"

echo "2. overdue filter"
curl -sS "${AUTH[@]}" "$BASE/repairs?status=overdue" | grep -q "$JOB" || fail "job missing from overdue filter"
pass "past promise appears as overdue"

echo "3. audited passcode reveal"
CODE=$(curl -sS -X POST "$BASE/repairs/$JOB/passcode/reveal" "${AUTH[@]}" | jqr "d['passcode']")
[ "$CODE" = "2580" ] || fail "encrypted passcode did not round trip"
pass "authorized reveal succeeded"

echo "4. QC is mandatory before completion"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" -d '{"status":"in_progress"}' > /dev/null
HTTP=$(curl -sS -o /tmp/qc-required.json -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"completed","labor_amount":2000}')
[ "$HTTP" = "409" ] || fail "completion bypassed QC (HTTP $HTTP)"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"ready_for_pickup","note":"screen, touch, charging and camera passed"}' > /dev/null
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"completed","labor_amount":2000}' > /dev/null
pass "QC gate enforced and valid path completed"

echo "5. linked warranty rework"
REWORK=$(curl -sS -X POST "$BASE/repairs/$JOB/rework" "${AUTH[@]}" \
  -d '{"reason":"touch stopped responding under warranty"}')
[ "$(printf '%s' "$REWORK" | jqr "d['parent_job_id']")" = "$JOB" ] || fail "rework missing parent link"
[ "$(printf '%s' "$REWORK" | jqr "d['labor_amount']")" = "0" ] || fail "rework should start at zero charge"
pass "rework linked to original job"

echo "Repair workflow smoke test passed."
