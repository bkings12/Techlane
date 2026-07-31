#!/usr/bin/env bash
# Smoke test for the parts <-> repair status coupling:
#  - a job waiting on several parts stays in waiting_parts until the LAST one lands
#  - closing a job voids any part authorisation still outstanding on it
set -euo pipefail

BASE="${BASE:-http://localhost:8080/api/v1}"
EMAIL="${EMAIL:-owner@techlane.local}"
PASSWORD="${PASSWORD:-password}"

jqr() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval('d'+sys.argv[1]))" "$1"; }

TOKEN=$(curl -sS -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jqr "['tokens']['access_token']")
AUTH=(-H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json')

BRANCH=$(curl -sS "${AUTH[@]}" "$BASE/branches" | jqr "['items'][0]['id']")
SUPPLIER=$(curl -sS "${AUTH[@]}" "$BASE/suppliers" | jqr "['items'][0]['id']")

pass() { printf '  ok   %s\n' "$1"; }
fail() { printf '  FAIL %s\n' "$1"; exit 1; }

new_job() {
  local cust dev
  cust=$(curl -sS -X POST "$BASE/customers" "${AUTH[@]}" \
    -d "{\"full_name\":\"Parts Smoke\",\"phone\":\"07121$RANDOM\"}" | jqr "['id']")
  dev=$(curl -sS -X POST "$BASE/devices" "${AUTH[@]}" \
    -d "{\"customer_id\":\"$cust\",\"kind\":\"phone\",\"brand\":\"Test\",\"model\":\"Parts\"}" | jqr "['id']")
  curl -sS -X POST "$BASE/repairs" "${AUTH[@]}" -d "{
    \"branch_id\":\"$BRANCH\",\"customer_id\":\"$cust\",\"device_id\":\"$dev\",
    \"problem_summary\":\"needs two parts\",\"labor_amount\":6000}" | jqr "['id']"
}

request_part() {
  curl -sS -X POST "$BASE/part-requests" "${AUTH[@]}" -d "{
    \"repair_job_id\":\"$1\",\"branch_id\":\"$BRANCH\",\"description\":\"$2\",
    \"quantity\":1,\"supplier_id\":\"$SUPPLIER\"}" | jqr "['id']"
}

job_status() { curl -sS "${AUTH[@]}" "$BASE/repairs/$1" | jqr "['status']"; }

JOB=$(new_job)
P1=$(request_part "$JOB" "Screen assembly")
P2=$(request_part "$JOB" "Charging flex")

echo "1. requesting a part blocks the job on parts"
[ "$(job_status "$JOB")" = "waiting_parts" ] || fail "expected waiting_parts, got $(job_status "$JOB")"
pass "job is waiting_parts"

echo "2. approving both parts issues two supplier authorisations"
I1=$(curl -sS -X POST "$BASE/part-requests/$P1/approve" "${AUTH[@]}" -d "{\"supplier_id\":\"$SUPPLIER\",\"unit_cost\":3200}")
I2=$(curl -sS -X POST "$BASE/part-requests/$P2/approve" "${AUTH[@]}" -d "{\"supplier_id\":\"$SUPPLIER\",\"unit_cost\":800}")
ID1=$(echo "$I1" | jqr "['id']"); CODE1=$(echo "$I1" | jqr "['auth_code']")
ID2=$(echo "$I2" | jqr "['id']"); CODE2=$(echo "$I2" | jqr "['auth_code']")
[ -n "$CODE1" ] && [ -n "$CODE2" ] || fail "missing auth codes"
pass "two auth codes issued"

echo "3. collecting only the FIRST part must NOT put the job back on the bench"
curl -sS -X POST "$BASE/supplier-issues/$ID1/collect" "${AUTH[@]}" -d "{\"auth_code\":\"$CODE1\"}" > /dev/null
ST=$(job_status "$JOB")
[ "$ST" = "waiting_parts" ] || fail "job jumped to $ST while a part is still outstanding"
TL=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB" | python3 -c "import sys,json;print(' | '.join((e.get('note') or '') for e in json.load(sys.stdin)['timeline']))")
case "$TL" in *"1 more part still outstanding"*) ;; *) fail "timeline did not record the partial arrival: $TL";; esac
pass "still waiting_parts, partial arrival recorded on the timeline"

echo "4. collecting the LAST part returns the job to the bench automatically"
curl -sS -X POST "$BASE/supplier-issues/$ID2/collect" "${AUTH[@]}" -d "{\"auth_code\":\"$CODE2\"}" > /dev/null
ST=$(job_status "$JOB")
[ "$ST" = "in_progress" ] || fail "expected in_progress after last part, got $ST"
pass "auto-returned to in_progress"

echo "5. closing a job voids the part authorisations still outstanding on it"
JOB2=$(new_job)
P3=$(request_part "$JOB2" "Back glass")
I3=$(curl -sS -X POST "$BASE/part-requests/$P3/approve" "${AUTH[@]}" -d "{\"supplier_id\":\"$SUPPLIER\",\"unit_cost\":1500}")
ID3=$(echo "$I3" | jqr "['id']"); CODE3=$(echo "$I3" | jqr "['auth_code']")
curl -sS -X POST "$BASE/repairs/$JOB2/status" "${AUTH[@]}" \
  -d '{"status":"unrepairable","closure_reason":"further_damage_found"}' > /dev/null
sleep 1
PR=$(curl -sS "${AUTH[@]}" "$BASE/part-requests?repair_job_id=$JOB2" | jqr "['items'][0]['status']")
[ "$PR" = "cancelled" ] || fail "part request still $PR after the job was written off"
CODE=$(curl -sS -o /tmp/redeem.json -w '%{http_code}' -X POST "$BASE/supplier-issues/$ID3/collect" "${AUTH[@]}" \
  -d "{\"auth_code\":\"$CODE3\"}")
[ "$CODE" != "200" ] || fail "a voided auth code was still redeemable — this is the leak"
pass "request cancelled and the auth code is dead ($CODE)"

echo
echo "parts flow smoke test passed"
