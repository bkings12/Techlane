#!/usr/bin/env bash
# Smoke test for work authorisation and labour variance:
#  - a job cannot go on the bench until somebody agreed to a price
#  - a customer approving an estimate is that agreement
#  - a manager can record a walk-in go-ahead instead
#  - charging more than was agreed requires a written reason
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

CUST=""
new_job() {
  local dev
  CUST=$(curl -sS -X POST "$BASE/customers" "${AUTH[@]}" \
    -d "{\"full_name\":\"Auth Smoke\",\"phone\":\"07122$RANDOM\"}" | jqr "['id']")
  dev=$(curl -sS -X POST "$BASE/devices" "${AUTH[@]}" \
    -d "{\"customer_id\":\"$CUST\",\"kind\":\"phone\",\"brand\":\"Test\",\"model\":\"Auth\"}" | jqr "['id']")
  curl -sS -X POST "$BASE/repairs" "${AUTH[@]}" -d "{
    \"branch_id\":\"$BRANCH\",\"customer_id\":\"$CUST\",\"device_id\":\"$dev\",
    \"problem_summary\":\"screen cracked\"}" | jqr "['id']"
}
job_status() { curl -sS "${AUTH[@]}" "$BASE/repairs/$1" | jqr "['status']"; }

echo "1. an unauthorized job cannot go on the bench"
JOB=$(new_job)
CODE=$(curl -sS -o /tmp/auth_block.json -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"in_progress"}')
[ "$CODE" = "409" ] || fail "expected 409, got $CODE: $(cat /tmp/auth_block.json)"
grep -q WORK_NOT_AUTHORIZED /tmp/auth_block.json || fail "expected WORK_NOT_AUTHORIZED"
pass "blocked with WORK_NOT_AUTHORIZED"

echo "2. a manager can record a walk-in go-ahead, which unblocks the bench"
CODE=$(curl -sS -o /tmp/auth_nonote.json -w '%{http_code}' -X POST "$BASE/repairs/$JOB/authorize-work" "${AUTH[@]}" \
  -d '{"amount":5000}')
[ "$CODE" = "400" ] || fail "authorizing without a note should be rejected, got $CODE"
curl -sS -X POST "$BASE/repairs/$JOB/authorize-work" "${AUTH[@]}" \
  -d '{"amount":5000,"note":"verbal go-ahead at the counter"}' > /tmp/authorized.json
[ "$(jqr "['authorization']['source']" < /tmp/authorized.json)" = "manager_override" ] || fail "source not recorded"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" -d '{"status":"in_progress"}' > /dev/null
[ "$(job_status "$JOB")" = "in_progress" ] || fail "still blocked after authorization"
pass "note required; go-ahead recorded and bench unblocked"

echo "3. charging more than agreed requires a written reason"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" -d '{"status":"ready_for_pickup","note":"QC passed"}' > /dev/null
CODE=$(curl -sS -o /tmp/variance.json -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"completed","labor_amount":8000}')
[ "$CODE" = "409" ] || fail "expected 409 for an unexplained overrun, got $CODE"
grep -q VARIANCE_REASON_REQUIRED /tmp/variance.json || fail "expected VARIANCE_REASON_REQUIRED"
pass "unexplained overrun blocked"

echo "4. charging LESS than agreed needs no explanation"
JOB3=$(new_job)
curl -sS -X POST "$BASE/repairs/$JOB3/authorize-work" "${AUTH[@]}" \
  -d '{"amount":5000,"note":"walk-in"}' > /dev/null
curl -sS -X POST "$BASE/repairs/$JOB3/status" "${AUTH[@]}" -d '{"status":"in_progress"}' > /dev/null
curl -sS -X POST "$BASE/repairs/$JOB3/status" "${AUTH[@]}" -d '{"status":"ready_for_pickup","note":"QC passed"}' > /dev/null
curl -sS -X POST "$BASE/repairs/$JOB3/status" "${AUTH[@]}" -d '{"status":"completed","labor_amount":3500}' > /dev/null
[ "$(job_status "$JOB3")" = "completed" ] || fail "a discount should not be blocked"
pass "discount allowed without a reason"

echo "5. an explained overrun goes through and lands on the timeline"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"completed","labor_amount":8000,"variance_reason":"board-level fault found, customer approved by phone"}' \
  > /tmp/variance_ok.json
[ "$(jqr "['status']" < /tmp/variance_ok.json)" = "completed" ] || fail "explained overrun was rejected"
DETAIL=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB")
echo "$DETAIL" | grep -q "board-level fault found" || fail "variance reason not persisted"
TL=$(echo "$DETAIL" | python3 -c "import sys,json;print(' | '.join((e.get('note') or '') for e in json.load(sys.stdin)['timeline']))")
case "$TL" in *"exceeds authorized"*) ;; *) fail "overrun not recorded on the timeline: $TL";; esac
pass "explained overrun recorded on the job and timeline"

echo "6. a customer approving an estimate is itself the authorization"
JOB2=$(new_job)
EST=$(curl -sS -X POST "$BASE/repairs/$JOB2/estimates" "${AUTH[@]}" \
  -d '{"labor_amount":4200,"parts_amount":1800,"notes":"screen + labour"}' | jqr "['id']")
[ "$(job_status "$JOB2")" = "diagnosed" ] || fail "creating an estimate should move the job to diagnosed"

# The OTP is hashed at rest, so mint a customer session directly rather than
# trying to read the code. Everything after this point is the real API.
CTOKEN="smoke-$RANDOM$RANDOM"
CHASH=$(printf '%s' "$CTOKEN" | sha256sum | cut -d' ' -f1)
# new_job runs in a subshell, so read the owning customer back off the job.
CUST=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB2" | jqr "['customer_id']")
TENANT=$(PGPASSWORD=techlane psql -h localhost -p 5433 -U techlane -d techlane -tAc \
  "SELECT tenant_id FROM repair.repair_jobs WHERE id = '$JOB2'")
PGPASSWORD=techlane psql -h localhost -p 5433 -U techlane -d techlane -q -c \
  "INSERT INTO repair.customer_sessions (id, tenant_id, customer_id, token_hash, expires_at)
   VALUES (gen_random_uuid(), '$TENANT', '$CUST', '$CHASH', now() + interval '1 hour')"

curl -sS -X POST "$BASE/customer/repairs/$JOB2/estimates/$EST/approve" \
  -H "Authorization: Bearer $CTOKEN" -H 'Content-Type: application/json' > /tmp/cust_approve.json
[ "$(jqr "['status']" < /tmp/cust_approve.json)" = "approved" ] || fail "customer approval failed: $(cat /tmp/cust_approve.json)"

DETAIL=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB2")
SRC=$(echo "$DETAIL" | jqr "['authorization']['source']")
[ "$SRC" = "customer_estimate" ] || fail "expected customer_estimate authorization, got $SRC"
AMT=$(echo "$DETAIL" | jqr "['authorization']['authorized_amount']")
[ "$(python3 -c "print(float('$AMT') == 4200)")" = "True" ] || fail "authorized amount should be the approved labour, got $AMT"
[ "$(job_status "$JOB2")" = "in_progress" ] || fail "approval should put the job on the bench, got $(job_status "$JOB2")"
pass "customer approval authorized the work and started the job"

echo
echo "authorization smoke test passed"
