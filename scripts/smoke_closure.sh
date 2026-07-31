#!/usr/bin/env bash
# Smoke test for repair job closure states (cancelled / unrepairable).
# Requires a running platform on $BASE with the seeded owner account.
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

new_job() {
  local cust dev
  cust=$(curl -sS -X POST "$BASE/customers" "${AUTH[@]}" \
    -d "{\"full_name\":\"Closure Smoke\",\"phone\":\"07120$RANDOM\"}" | jqr "['id']")
  dev=$(curl -sS -X POST "$BASE/devices" "${AUTH[@]}" \
    -d "{\"customer_id\":\"$cust\",\"kind\":\"phone\",\"brand\":\"Test\",\"model\":\"Closure\"}" | jqr "['id']")
  curl -sS -X POST "$BASE/repairs" "${AUTH[@]}" -d "{
    \"branch_id\": \"$BRANCH\",
    \"customer_id\": \"$cust\",
    \"device_id\": \"$dev\",
    \"problem_summary\": \"smoke test\",
    \"labor_amount\": 4500
  }" | jqr "['id']"
}

echo "1. closing without a reason is rejected"
JOB=$(new_job)
CODE=$(curl -sS -o /tmp/close_noreason.json -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"cancelled"}')
[ "$CODE" = "400" ] || fail "expected 400, got $CODE: $(cat /tmp/close_noreason.json)"
grep -q INVALID_CLOSURE /tmp/close_noreason.json || fail "expected INVALID_CLOSURE code"
pass "reason is mandatory (400 INVALID_CLOSURE)"

echo "2. an unknown reason code is rejected"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"cancelled","closure_reason":"because_i_said_so"}')
[ "$CODE" = "400" ] || fail "expected 400 for unknown reason, got $CODE"
pass "unknown reason codes rejected"

echo "3. a write-off reason cannot be used to cancel"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"cancelled","closure_reason":"beyond_economical_repair"}')
[ "$CODE" = "400" ] || fail "expected 400 for cross-status reason, got $CODE"
pass "reason codes are scoped to their outcome"

echo "4. cancelling clears the quoted price so the device can be handed back"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"cancelled","closure_reason":"customer_declined_quote","note":"quote too high"}' > /tmp/cancelled.json
[ "$(jqr "['status']" < /tmp/cancelled.json)" = "cancelled" ] || fail "status did not change"
LABOR=$(jqr "['labor_amount']" < /tmp/cancelled.json)
[ "$(python3 -c "print(float('$LABOR') == 0)")" = "True" ] || fail "labor should be cleared, got $LABOR"
[ "$(jqr "['closure_reason']" < /tmp/cancelled.json)" = "customer_declined_quote" ] || fail "reason not persisted"
pass "cancelled, price cleared, reason persisted"

echo "5. a cancelled job can still be handed back, but nothing else"
NEXT=$(curl -sS "${AUTH[@]}" "$BASE/repairs/$JOB" | jqr "['next_statuses']")
[ "$NEXT" = "['collected']" ] || fail "expected only ['collected'], got $NEXT"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" \
  -d '{"status":"in_progress"}')
[ "$CODE" = "409" ] || fail "expected 409 reopening a closed job, got $CODE"
curl -sS -X POST "$BASE/repairs/$JOB/status" "${AUTH[@]}" -d '{"status":"collected"}' > /tmp/returned.json
[ "$(jqr "['status']" < /tmp/returned.json)" = "collected" ] || fail "device return blocked"
pass "device returned; job cannot be reopened"

echo "6. unrepairable write-off works from in_progress"
JOB2=$(new_job)
curl -sS -X POST "$BASE/repairs/$JOB2/status" "${AUTH[@]}" -d '{"status":"in_progress"}' > /dev/null
curl -sS -X POST "$BASE/repairs/$JOB2/status" "${AUTH[@]}" \
  -d '{"status":"unrepairable","closure_reason":"beyond_economical_repair"}' > /tmp/writeoff.json
[ "$(jqr "['status']" < /tmp/writeoff.json)" = "unrepairable" ] || fail "write-off failed"
pass "written off as unrepairable"

echo "7. closed jobs are excluded from open counts and appear in the loss report"
CLOSED=$(curl -sS "${AUTH[@]}" "$BASE/repairs?status=closed" | python3 -c "import sys,json;print(len(json.load(sys.stdin)['items']))")
[ "$CLOSED" -ge 1 ] || fail "closed filter returned nothing"
OPEN_IDS=$(curl -sS "${AUTH[@]}" "$BASE/repairs?status=open" | python3 -c "import sys,json;print(' '.join(i['id'] for i in json.load(sys.stdin)['items']))")
case " $OPEN_IDS " in *" $JOB2 "*) fail "written-off job still counted as open";; esac
REASONS=$(curl -sS "${AUTH[@]}" "$BASE/reports/operations?days=7" | python3 -c "import sys,json;print(','.join(c['reason'] for c in json.load(sys.stdin)['closures']))")
case "$REASONS" in *beyond_economical_repair*) ;; *) fail "loss report missing the write-off: $REASONS";; esac
pass "open counts exclude closures; loss report groups by reason"

echo
echo "closure smoke test passed"
