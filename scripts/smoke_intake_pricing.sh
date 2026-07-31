#!/usr/bin/env bash
# Smoke test for the two intake pricing paths:
#  - price agreed at the counter authorises the work immediately
#  - diagnose-first leaves the job unpriced and unauthorised until an estimate lands
set -euo pipefail

BASE="${BASE:-http://localhost:8080/api/v1}"
EMAIL="${EMAIL:-owner@techlane.local}"
PASSWORD="${PASSWORD:-password}"

jqr() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval('d'+sys.argv[1]))" "$1"; }

TOKEN=$(curl -sS -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jqr "['tokens']['access_token']")
AUTH=(-H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json')
BRANCH=$(curl -sS "${AUTH[@]}" "$BASE/branches" | jqr "['items'][0]['id']")

pass() { printf '  ok   %s\n' "$1"; }
fail() { printf '  FAIL %s\n' "$1"; exit 1; }

# $1 = labor_amount json fragment ("" to omit)
new_job() {
  local cust dev extra=""
  cust=$(curl -sS -X POST "$BASE/customers" "${AUTH[@]}" \
    -d "{\"full_name\":\"Intake Smoke\",\"phone\":\"07123$RANDOM\"}" | jqr "['id']")
  dev=$(curl -sS -X POST "$BASE/devices" "${AUTH[@]}" \
    -d "{\"customer_id\":\"$cust\",\"kind\":\"phone\",\"brand\":\"Test\",\"model\":\"Intake\"}" | jqr "['id']")
  [ -n "$1" ] && extra=",\"labor_amount\":$1"
  curl -sS -X POST "$BASE/repairs" "${AUTH[@]}" -d "{
    \"branch_id\":\"$BRANCH\",\"customer_id\":\"$cust\",\"device_id\":\"$dev\",
    \"problem_summary\":\"wont charge\"$extra}" | jqr "['id']"
}

echo "1. a price agreed at the counter authorises the work"
JOB=$(new_job 5500)
D=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB")
[ "$(echo "$D" | jqr "['authorization']['source']")" = "intake_agreed" ] || fail "intake price did not authorise the job"
AMT=$(echo "$D" | jqr "['authorization']['authorized_amount']")
[ "$(python3 -c "print(float('$AMT') == 5500)")" = "True" ] || fail "authorized amount wrong: $AMT"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" -d '{"status":"in_progress"}' > /dev/null
[ "$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB" | jqr "['status']")" = "in_progress" ] || fail "bench blocked despite agreed price"
pass "authorised at intake and work can start"

echo "2. intake without a price creates an unpriced, unauthorised job"
JOB2=$(new_job "")
D2=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB2")
LA=$(echo "$D2" | jqr "['labor_amount']")
[ "$(python3 -c "print(float('$LA') == 0)")" = "True" ] || fail "expected no price, got $LA"
python3 -c "
import json,sys
d=json.load(open('/dev/stdin'))
a=d.get('authorization') or {}
sys.exit(0 if not a.get('authorized_at') else 1)
" <<< "$D2" || fail "a job with no price must not be authorised"
pass "no price, no authorisation"

echo "3. that job is blocked from the bench until a price is agreed"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE/repairs/$JOB2/status" "${AUTH[@]}" \
  -d '{"status":"in_progress"}')
[ "$CODE" = "409" ] || fail "expected 409, got $CODE"
pass "bench blocked until the price is agreed"

echo "4. the diagnose-then-quote path unblocks it"
curl -sS -X POST "$BASE/repairs/$JOB2/estimates" "${AUTH[@]}" \
  -d '{"labor_amount":3000,"parts_amount":2500,"notes":"charging port + labour"}' > /dev/null
[ "$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB2" | jqr "['status']")" = "diagnosed" ] || fail "estimate should move the job to diagnosed"
TL=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB2" | python3 -c "import sys,json;print(' | '.join((e.get('note') or '') for e in json.load(sys.stdin)['timeline']))")
case "$TL" in *"diagnose first"*) ;; *) fail "intake note should say the price is pending: $TL";; esac
pass "diagnosed with an estimate out for approval"

echo
echo "intake pricing smoke test passed"
