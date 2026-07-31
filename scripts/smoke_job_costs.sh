#!/usr/bin/env bash
# Walks a repair job through "part issued from our own shelf" and checks that
# stock drops, the cost lands on the job, and margin/reports reflect it.
set -euo pipefail

API="${API:-http://localhost:8080/api/v1}"
EMAIL="${EMAIL:-owner@techlane.local}"
PASSWORD="${PASSWORD:-password}"

jqr() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval(sys.argv[1]))" "$1"; }
say() { printf '\n\033[1m%s\033[0m\n' "$*"; }
ok()  { printf '  \033[32m✓\033[0m %s\n' "$*"; }
die() { printf '  \033[31m✗ %s\033[0m\n' "$*"; exit 1; }

say "Login"
TOKEN=$(curl -sS -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jqr "d['tokens']['access_token']")
[ -n "$TOKEN" ] || die "no token"
AUTH=(-H "Authorization: Bearer $TOKEN")
ok "authenticated"

BRANCH=$(curl -sS "${AUTH[@]}" "$API/branches" | jqr "d['items'][0]['id']")

say "Find a stock line with quantity on hand and give it a cost price"
BAL=$(curl -sS "${AUTH[@]}" "$API/inventory/balances")
VARIANT=$(echo "$BAL" | jqr "[b for b in d['items'] if b['available_qty']>0][0]['variant_id']")
LOCATION=$(echo "$BAL" | jqr "[b for b in d['items'] if b['available_qty']>0][0]['location_id']")
QTY_BEFORE=$(echo "$BAL" | jqr "[b for b in d['items'] if b['available_qty']>0][0]['available_qty']")
UNIT_COST=1250
curl -sS "${AUTH[@]}" -X PATCH "$API/variants/$VARIANT" -H 'Content-Type: application/json' \
  -d "{\"cost_price\":$UNIT_COST}" > /dev/null
SET_COST=$(curl -sS "${AUTH[@]}" "$API/inventory/balances?location_id=$LOCATION" \
  | jqr "[b for b in d['items'] if b['variant_id']=='$VARIANT'][0]['cost_price']")
python3 -c "assert float('$SET_COST') == $UNIT_COST" || die "cost price did not stick (got $SET_COST)"
ok "variant ${VARIANT:0:8} at ${LOCATION:0:8}, $QTY_BEFORE on hand, cost $SET_COST"

say "Create a job with an agreed price"
CUST=$(curl -sS "${AUTH[@]}" -X POST "$API/customers" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Cost Test $RANDOM\",\"phone\":\"07122$RANDOM\"}" | jqr "d['id']")
DEV=$(curl -sS "${AUTH[@]}" -X POST "$API/devices" -H 'Content-Type: application/json' \
  -d "{\"customer_id\":\"$CUST\",\"kind\":\"phone\",\"brand\":\"Tecno\",\"model\":\"Spark\"}" | jqr "d['id']")
JOB=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs" -H 'Content-Type: application/json' \
  -d "{\"branch_id\":\"$BRANCH\",\"customer_id\":\"$CUST\",\"device_id\":\"$DEV\",\"problem_summary\":\"screen cracked\",\"labor_amount\":5000}" \
  | jqr "d['id']")
ok "job ${JOB:0:8} at KES 5000 labor"

say "Request a part, then fulfil it from our own stock"
PR=$(curl -sS "${AUTH[@]}" -X POST "$API/part-requests" -H 'Content-Type: application/json' \
  -d "{\"repair_job_id\":\"$JOB\",\"branch_id\":\"$BRANCH\",\"description\":\"Spark screen\",\"quantity\":1}" \
  | jqr "d['id']")
RES=$(curl -sS "${AUTH[@]}" -X POST "$API/part-requests/$PR/issue-from-stock" -H 'Content-Type: application/json' \
  -d "{\"variant_id\":\"$VARIANT\",\"location_id\":\"$LOCATION\",\"quantity\":1}")
STATUS=$(echo "$RES" | jqr "d.get('status','')")
[ "$STATUS" = "issued_from_stock" ] || die "expected issued_from_stock, got '$STATUS' ($RES)"
ok "part request marked issued_from_stock"

say "Stock should have dropped by one"
QTY_AFTER=$(curl -sS "${AUTH[@]}" "$API/inventory/balances?location_id=$LOCATION" \
  | jqr "[b for b in d['items'] if b['variant_id']=='$VARIANT'][0]['available_qty']")
EXPECTED=$((QTY_BEFORE - 1))
[ "$QTY_AFTER" = "$EXPECTED" ] || die "expected $EXPECTED on hand, got $QTY_AFTER"
ok "on hand went $QTY_BEFORE → $QTY_AFTER"

say "Cost should be booked against the job"
MARGIN=$(curl -sS "${AUTH[@]}" "$API/repairs/$JOB/margin")
COSTS=$(echo "$MARGIN" | jqr "len(d['costs'])")
[ "$COSTS" -ge 1 ] || die "no cost entries booked ($MARGIN)"
LABOR=$(echo "$MARGIN" | jqr "float(d['labor_amount'])")
PARTS=$(echo "$MARGIN" | jqr "float(d['parts_cost'])")
MARG=$(echo "$MARGIN" | jqr "float(d['margin'])")
UNPRICED=$(echo "$MARGIN" | jqr "d['unpriced_parts']")
python3 -c "assert abs($LABOR - $PARTS - $MARG) < 0.01, 'margin math wrong'" || die "margin != labor - parts"
python3 -c "assert abs($PARTS - $UNIT_COST) < 0.01, 'parts cost not the buying price'" \
  || die "expected parts cost $UNIT_COST, got $PARTS"
[ "$UNPRICED" = "0" ] || die "part was flagged unpriced even though a cost price is set"
ok "labor $LABOR − parts $PARTS = margin $MARG, $COSTS cost line(s), 0 unpriced"

say "Issuing the same request twice must not double-book"
AGAIN=$(curl -sS -o /dev/null -w '%{http_code}' "${AUTH[@]}" -X POST "$API/part-requests/$PR/issue-from-stock" \
  -H 'Content-Type: application/json' -d "{\"variant_id\":\"$VARIANT\",\"location_id\":\"$LOCATION\",\"quantity\":1}")
[ "$AGAIN" != "200" ] || die "second issue was accepted (HTTP $AGAIN)"
ok "rejected with HTTP $AGAIN"

say "A part with no cost price must be flagged, not counted as free"
curl -sS "${AUTH[@]}" -X PATCH "$API/variants/$VARIANT" -H 'Content-Type: application/json' \
  -d '{"cost_price":0}' > /dev/null
JOB_B=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs" -H 'Content-Type: application/json' \
  -d "{\"branch_id\":\"$BRANCH\",\"customer_id\":\"$CUST\",\"device_id\":\"$DEV\",\"problem_summary\":\"no cost part\",\"labor_amount\":3000}" \
  | jqr "d['id']")
PR_B=$(curl -sS "${AUTH[@]}" -X POST "$API/part-requests" -H 'Content-Type: application/json' \
  -d "{\"repair_job_id\":\"$JOB_B\",\"branch_id\":\"$BRANCH\",\"description\":\"Unpriced part\",\"quantity\":1}" \
  | jqr "d['id']")
curl -sS "${AUTH[@]}" -X POST "$API/part-requests/$PR_B/issue-from-stock" -H 'Content-Type: application/json' \
  -d "{\"variant_id\":\"$VARIANT\",\"location_id\":\"$LOCATION\",\"quantity\":1}" > /dev/null
FLAG=$(curl -sS "${AUTH[@]}" "$API/repairs/$JOB_B/margin" | jqr "d['unpriced_parts']")
[ "$FLAG" = "1" ] || die "expected 1 unpriced part, got $FLAG"
ok "margin reports 1 unpriced part instead of pretending the part was free"

say "Partial issue against a multi-quantity request must be refused"
PR_C=$(curl -sS "${AUTH[@]}" -X POST "$API/part-requests" -H 'Content-Type: application/json' \
  -d "{\"repair_job_id\":\"$JOB_B\",\"branch_id\":\"$BRANCH\",\"description\":\"Two screws\",\"quantity\":2}" \
  | jqr "d['id']")
SHORT=$(curl -sS -o /dev/null -w '%{http_code}' "${AUTH[@]}" -X POST "$API/part-requests/$PR_C/issue-from-stock" \
  -H 'Content-Type: application/json' -d "{\"variant_id\":\"$VARIANT\",\"location_id\":\"$LOCATION\",\"quantity\":1}")
[ "$SHORT" != "200" ] || die "a short issue closed a request needing 2"
ok "short issue rejected with HTTP $SHORT"

say "Complete the job and check it shows in Reports profitability"
curl -sS "${AUTH[@]}" -X POST "$API/repairs/$JOB/status" -H 'Content-Type: application/json' \
  -d '{"status":"in_progress","note":"on bench"}' > /dev/null
curl -sS "${AUTH[@]}" -X POST "$API/repairs/$JOB/status" -H 'Content-Type: application/json' \
  -d '{"status":"ready_for_pickup","note":"QC passed"}' > /dev/null
curl -sS "${AUTH[@]}" -X POST "$API/repairs/$JOB/status" -H 'Content-Type: application/json' \
  -d '{"status":"completed","note":"done","labor_amount":5000}' > /dev/null
PROF=$(curl -sS "${AUTH[@]}" "$API/reports/operations?days=7" | jqr "json.dumps(d['repair_profitability'])")
echo "  $PROF"
JOBS=$(echo "$PROF" | jqr "d['jobs']")
[ "$JOBS" -ge 1 ] || die "profitability shows no finished jobs"
ok "reports rolled up $JOBS finished job(s)"

printf '\n\033[32mAll job-cost checks passed.\033[0m\n'
