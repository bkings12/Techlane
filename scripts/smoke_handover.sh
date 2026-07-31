#!/usr/bin/env bash
# Handover verification at collection: a device only leaves the counter once we
# have recorded who took it and how we established they were entitled to it.
set -euo pipefail

API="${API:-http://localhost:8080/api/v1}"
EMAIL="${EMAIL:-owner@techlane.local}"
PASSWORD="${PASSWORD:-password}"
PGURL="${PGURL:-postgres://techlane:techlane@localhost:5433/techlane?sslmode=disable}"

jqr() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval(sys.argv[1]))" "$1"; }
say() { printf '\n\033[1m%s\033[0m\n' "$*"; }
ok()  { printf '  \033[32m✓\033[0m %s\n' "$*"; }
die() { printf '  \033[31m✗ %s\033[0m\n' "$*"; exit 1; }
psqlq() { PGPASSWORD=techlane psql -h localhost -p 5433 -U techlane -d techlane -tAc "$1"; }

say "Login"
TOKEN=$(curl -sS -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jqr "d['tokens']['access_token']")
[ -n "$TOKEN" ] || die "no token"
AUTH=(-H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json')
BRANCH=$(curl -sS "${AUTH[@]}" "$API/branches" | jqr "d['items'][0]['id']")
ok "authenticated"

# A finished job, paid in full, ready to walk out the door.
finished_job() {
  local phone="07133$RANDOM" cust dev job
  cust=$(curl -sS "${AUTH[@]}" -X POST "$API/customers" \
    -d "{\"full_name\":\"Handover Test\",\"phone\":\"$phone\"}" | jqr "d['id']")
  dev=$(curl -sS "${AUTH[@]}" -X POST "$API/devices" \
    -d "{\"customer_id\":\"$cust\",\"kind\":\"phone\",\"brand\":\"Test\",\"model\":\"Handover\"}" | jqr "d['id']")
  job=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs" \
    -d "{\"branch_id\":\"$BRANCH\",\"customer_id\":\"$cust\",\"device_id\":\"$dev\",\"problem_summary\":\"screen\",\"labor_amount\":2000}" \
    | jqr "d['id']")
  curl -sS "${AUTH[@]}" -X POST "$API/repairs/$job/status" -d '{"status":"in_progress"}' > /dev/null
  curl -sS "${AUTH[@]}" -X POST "$API/repairs/$job/status" \
    -d '{"status":"ready_for_pickup","note":"QC passed"}' > /dev/null
  curl -sS "${AUTH[@]}" -X POST "$API/repairs/$job/status" \
    -d '{"status":"completed","labor_amount":2000}' > /dev/null
  echo "$job"
}

pay_in_full() {
  curl -sS "${AUTH[@]}" -X POST "$API/payments" \
    -d "{\"branch_id\":\"$BRANCH\",\"method\":\"cash\",\"amount\":2000,\"payable_type\":\"repair\",\"payable_id\":\"$1\"}" \
    > /dev/null
}

say "Setting status to collected directly must be refused"
JOB=$(finished_job)
pay_in_full "$JOB"
BODY=$(curl -sS -o /tmp/h1.json -w '%{http_code}' "${AUTH[@]}" -X POST "$API/repairs/$JOB/status" \
  -d '{"status":"collected"}')
[ "$BODY" = "409" ] || die "expected 409, got $BODY: $(cat /tmp/h1.json)"
grep -q HANDOVER_REQUIRED /tmp/h1.json || die "wrong error: $(cat /tmp/h1.json)"
ok "rejected with HANDOVER_REQUIRED"

say "Paying in full must not silently mark the device collected"
STATUS=$(curl -sS "${AUTH[@]}" "$API/repairs/$JOB" | jqr "d['status']")
[ "$STATUS" = "completed" ] || die "job became '$STATUS' just from being paid"
NOTED=$(curl -sS "${AUTH[@]}" "$API/repairs/$JOB" \
  | jqr "any('waiting for the customer to collect' in (e.get('note') or '') for e in d['timeline'])")
[ "$NOTED" = "True" ] || die "no timeline note that the job is paid and awaiting collection"
ok "still 'completed', with a note that it is paid and awaiting collection"

say "Releasing without a code needs authority, and is recorded as a vouch"
H=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs/$JOB/handover" \
  -d '{"collected_by_name":"Asha Mwangi","relationship":"self","note":"known to us"}')
METHOD=$(echo "$H" | jqr "d['verification_method']")
[ "$METHOD" = "staff_vouched" ] || die "expected staff_vouched, got '$METHOD' ($H)"
STATUS=$(curl -sS "${AUTH[@]}" "$API/repairs/$JOB" | jqr "d['status']")
[ "$STATUS" = "collected" ] || die "job did not become collected, got '$STATUS'"
ok "owner released it as a vouch, job is now collected"

say "The same device cannot be handed over twice"
AGAIN=$(curl -sS -o /tmp/h2.json -w '%{http_code}' "${AUTH[@]}" -X POST "$API/repairs/$JOB/handover" \
  -d '{"collected_by_name":"Someone Else"}')
[ "$AGAIN" = "409" ] || die "second handover returned $AGAIN"
ok "rejected with HTTP $AGAIN"

say "Sending a code with no SMS provider fails without stranding a challenge"
JOB2=$(finished_job)
pay_in_full "$JOB2"
SENT=$(curl -sS -o /tmp/h3.json -w '%{http_code}' "${AUTH[@]}" -X POST "$API/repairs/$JOB2/handover/send-code" -d '{}')
if [ "$SENT" = "202" ]; then
  ok "code sent via the configured provider"
else
  grep -qi "sms provider not configured" /tmp/h3.json || die "unexpected send error: $(cat /tmp/h3.json)"
  STRANDED=$(psqlq "SELECT COUNT(*) FROM repair.customer_otp_challenges WHERE repair_job_id = '$JOB2';")
  [ "$STRANDED" = "0" ] || die "$STRANDED challenge(s) left behind after a failed send"
  ok "send failed cleanly and left no challenge behind, so the counter can retry"
fi

# Codes are only ever stored hashed and are deliberately never logged, so the test
# seeds one the way the SMS provider would have delivered it.
say "A code the owner confirms releases the device and is recorded as verified"
PHONE=$(psqlq "SELECT c.phone FROM repair.customers c JOIN repair.repair_jobs j ON j.customer_id = c.id WHERE j.id = '$JOB2';")
CODE=424242
HASH=$(python3 -c "import hashlib;print(hashlib.sha256(b'$CODE').hexdigest())")
psqlq "INSERT INTO repair.customer_otp_challenges
         (id, tenant_id, phone_e164, code_hash, attempts, expires_at, created_at, purpose, repair_job_id)
       SELECT gen_random_uuid(), tenant_id, '$PHONE', '$HASH', 0, now() + interval '10 minutes', now(), 'handover', id
       FROM repair.repair_jobs WHERE id = '$JOB2';" > /dev/null

say "A wrong code is refused"
BAD=$(curl -sS -o /tmp/h4.json -w '%{http_code}' "${AUTH[@]}" -X POST "$API/repairs/$JOB2/handover" \
  -d '{"collected_by_name":"Asha","otp_code":"000000"}')
[ "$BAD" != "201" ] || die "a wrong code released the device"
grep -qi "invalid code" /tmp/h4.json || die "unexpected error: $(cat /tmp/h4.json)"
ok "rejected with HTTP $BAD (invalid code)"

# Checked while the code is still live and unused, so this proves purpose scoping
# rather than just re-rejecting a spent code.
say "A live handover code cannot be used to log in as the customer"
CUST_LOGIN=$(curl -sS -o /tmp/h7.json -w '%{http_code}' -X POST "$API/customer/auth/otp/verify" \
  -H 'Content-Type: application/json' -d "{\"phone\":\"$PHONE\",\"code\":\"$CODE\"}")
[ "$CUST_LOGIN" != "200" ] || die "a handover code logged in as the customer: $(cat /tmp/h7.json)"
ok "refused with HTTP $CUST_LOGIN — handover codes are scoped to handover only"

say "The right code releases it and records the method as 'otp'"
H2=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs/$JOB2/handover" \
  -d "{\"collected_by_name\":\"Asha Mwangi\",\"relationship\":\"self\",\"otp_code\":\"$CODE\"}")
M2=$(echo "$H2" | jqr "d.get('verification_method','')")
[ "$M2" = "otp" ] || die "expected otp, got '$M2' ($H2)"
ok "released against the owner's code"

say "The code is consumed so it cannot release anything again"
USED=$(psqlq "SELECT COUNT(*) FROM repair.customer_otp_challenges WHERE repair_job_id = '$JOB2' AND consumed_at IS NOT NULL;")
[ "$USED" = "1" ] || die "code was not consumed"
ok "code consumed"

say "A device with money still owed cannot be released"
JOB3=$(finished_job)
OWED=$(curl -sS -o /tmp/h5.json -w '%{http_code}' "${AUTH[@]}" -X POST "$API/repairs/$JOB3/handover" \
  -d '{"collected_by_name":"Asha Mwangi"}')
[ "$OWED" = "409" ] || die "expected 409 for unpaid job, got $OWED: $(cat /tmp/h5.json)"
grep -q BALANCE_DUE /tmp/h5.json || die "wrong error: $(cat /tmp/h5.json)"
ok "rejected with BALANCE_DUE"

say "A job still on the bench cannot be handed over"
CUST=$(curl -sS "${AUTH[@]}" -X POST "$API/customers" -d "{\"full_name\":\"Bench\",\"phone\":\"07144$RANDOM\"}" | jqr "d['id']")
DEV=$(curl -sS "${AUTH[@]}" -X POST "$API/devices" \
  -d "{\"customer_id\":\"$CUST\",\"kind\":\"phone\",\"brand\":\"T\",\"model\":\"B\"}" | jqr "d['id']")
JOB4=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs" \
  -d "{\"branch_id\":\"$BRANCH\",\"customer_id\":\"$CUST\",\"device_id\":\"$DEV\",\"problem_summary\":\"x\"}" | jqr "d['id']")
EARLY=$(curl -sS -o /tmp/h6.json -w '%{http_code}' "${AUTH[@]}" -X POST "$API/repairs/$JOB4/handover" \
  -d '{"collected_by_name":"Too Early"}')
[ "$EARLY" = "409" ] || die "expected 409 for a job in intake, got $EARLY: $(cat /tmp/h6.json)"
ok "rejected with HTTP $EARLY"

say "A third party collecting is recorded with their name and relationship"
JOB5=$(finished_job)
pay_in_full "$JOB5"
H5=$(curl -sS "${AUTH[@]}" -X POST "$API/repairs/$JOB5/handover" \
  -d '{"collected_by_name":"John Otieno","relationship":"brother","id_number":"12345678","note":"ID checked"}')
WHO=$(echo "$H5" | jqr "d['collected_by_name']")
REL=$(echo "$H5" | jqr "d['relationship']")
IDN=$(echo "$H5" | jqr "d['id_number']")
[ "$WHO" = "John Otieno" ] && [ "$REL" = "brother" ] && [ "$IDN" = "12345678" ] \
  || die "third-party details not recorded ($H5)"
TL=$(curl -sS "${AUTH[@]}" "$API/repairs/$JOB5" \
  | jqr "any('John Otieno (brother)' in (e.get('note') or '') for e in d['timeline'])")
[ "$TL" = "True" ] || die "handover not written to the timeline"
ok "recorded as John Otieno (brother), ID 12345678, and on the timeline"

printf '\n\033[32mAll handover checks passed.\033[0m\n'
