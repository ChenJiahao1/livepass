#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

source "${ROOT_DIR}/scripts/perf/lib/common.sh"
source "${ROOT_DIR}/scripts/perf/lib/mysql.sh"

RESULT_DIR="${1:-}"
ORDER_RPC_LOG_FILE="${ORDER_RPC_LOG_FILE:-${ROOT_DIR}/.codex/runlogs/order-rpc.log}"
SHOW_TIME_ID="${SHOW_TIME_ID:-30001}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-40001}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"
MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-livepass_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-livepass_order}"

[[ -n "${RESULT_DIR}" ]] || perf_fail "usage: bash scripts/perf/analyze_create_order_path.sh <result-dir>"
[[ -f "${RESULT_DIR}/summary.json" ]] || perf_fail "summary not found: ${RESULT_DIR}/summary.json"
[[ -f "${RESULT_DIR}/timing.json" ]] || perf_fail "timing not found: ${RESULT_DIR}/timing.json"

perf_require_cmd jq
perf_require_cmd python3

sum_regex_tables() {
  local db="$1"
  local table_regex="$2"
  local where_clause="$3"
  local total=0
  local table_names=()
  local table_name
  local count

  mapfile -t table_names < <(perf_mysql_query "${db}" "SELECT table_name FROM information_schema.tables WHERE table_schema = '${db}' AND table_name REGEXP '${table_regex}' ORDER BY table_name")

  for table_name in "${table_names[@]}"; do
    [[ -n "${table_name}" ]] || continue
    count="$(perf_mysql_query "${db}" "SELECT COUNT(*) FROM \`${table_name}\` WHERE ${where_clause}")"
    count="${count:-0}"
    total=$((total + count))
  done

  printf '%s\n' "${total}"
}

summary_count="$(jq '.createSuccessCount + .businessFailureCount + .inventoryInsufficientCount' "${RESULT_DIR}/summary.json")"

jq -n \
  --argjson count "${summary_count}" \
  --slurpfile timing "${RESULT_DIR}/timing.json" \
  '{
    count: $count,
    startEpoch: $timing[0].startEpoch,
    endEpoch: $timing[0].endEpoch,
    elapsedSeconds: $timing[0].elapsedSeconds,
    qpsByClientElapsed: (if $timing[0].elapsedSeconds > 0 then ($count / $timing[0].elapsedSeconds) else 0 end)
  }' \
  > "${RESULT_DIR}/client_qps.json"

if [[ -f "${ORDER_RPC_LOG_FILE}" ]]; then
  python3 - "${ORDER_RPC_LOG_FILE}" "${RESULT_DIR}/order_rpc_qps.json" <<'PY'
import json
import sys
from datetime import datetime

log_file = sys.argv[1]
output_file = sys.argv[2]
target = "create order perf stage"

timestamps = []
with open(log_file, "r", encoding="utf-8") as fh:
    for line in fh:
        line = line.strip()
        if not line:
            continue
        try:
            item = json.loads(line)
        except json.JSONDecodeError:
            continue
        if item.get("content") != target:
            continue
        ts = item.get("@timestamp")
        if not ts:
            continue
        timestamps.append(datetime.fromisoformat(ts))

payload = {
    "count": len(timestamps),
    "firstRequestAt": None,
    "lastResponseAt": None,
    "responseWindowSeconds": 0,
    "qpsByResponseWindow": 0,
}

if timestamps:
    first_ts = min(timestamps)
    last_ts = max(timestamps)
    elapsed = (last_ts - first_ts).total_seconds()
    payload.update({
        "firstRequestAt": first_ts.isoformat(),
        "lastResponseAt": last_ts.isoformat(),
        "responseWindowSeconds": elapsed,
        "qpsByResponseWindow": (len(timestamps) / elapsed) if elapsed > 0 else len(timestamps),
    })

with open(output_file, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
PY
else
  jq -n \
    '{count: 0, firstRequestAt: null, lastResponseAt: null, responseWindowSeconds: 0, qpsByResponseWindow: 0}' \
    > "${RESULT_DIR}/order_rpc_qps.json"
fi

jq -n \
  '{count: 0, firstRequestAt: null, firstResponseAt: null, lastResponseAt: null, requestToLastResponseSeconds: 0, responseWindowSeconds: 0, qpsByRequestToLastResponse: 0, qpsByResponseWindow: 0}' \
  > "${RESULT_DIR}/gateway_qps.json"

seat_available="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT COUNT(*) FROM d_seat WHERE show_time_id = ${SHOW_TIME_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND seat_status = 1 AND status = 1" || echo 0)"
seat_frozen="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT COUNT(*) FROM d_seat WHERE show_time_id = ${SHOW_TIME_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND seat_status = 2 AND status = 1" || echo 0)"
seat_sold="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT COUNT(*) FROM d_seat WHERE show_time_id = ${SHOW_TIME_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND seat_status = 3 AND status = 1" || echo 0)"
seat_occupied="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT COUNT(*) FROM d_seat WHERE show_time_id = ${SHOW_TIME_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND seat_status IN (2, 3) AND status = 1" || echo 0)"
ticket_category_total="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT total_number FROM d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND show_time_id = ${SHOW_TIME_ID} AND status = 1 LIMIT 1" || echo 0)"
ticket_category_remain="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT remain_number FROM d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND show_time_id = ${SHOW_TIME_ID} AND status = 1 LIMIT 1" || echo 0)"
order_rows="$(sum_regex_tables "${MYSQL_DB_ORDER}" '^d_order_[0-9]+$' "show_time_id = ${SHOW_TIME_ID} AND status = 1" || echo 0)"
order_ticket_rows="$(sum_regex_tables "${MYSQL_DB_ORDER}" '^d_order_ticket_user_[0-9]+$' "show_time_id = ${SHOW_TIME_ID} AND status = 1" || echo 0)"
user_guard_rows="$(sum_regex_tables "${MYSQL_DB_ORDER}" '^d_order_user_guard$' "show_time_id = ${SHOW_TIME_ID} AND status = 1" || echo 0)"
viewer_guard_rows="$(sum_regex_tables "${MYSQL_DB_ORDER}" '^d_order_viewer_guard$' "show_time_id = ${SHOW_TIME_ID} AND status = 1" || echo 0)"
seat_guard_rows="$(sum_regex_tables "${MYSQL_DB_ORDER}" '^d_order_seat_guard$' "show_time_id = ${SHOW_TIME_ID} AND status = 1" || echo 0)"

jq -n \
  --argjson seatAvailable "${seat_available:-0}" \
  --argjson seatFrozen "${seat_frozen:-0}" \
  --argjson seatSold "${seat_sold:-0}" \
  --argjson seatOccupied "${seat_occupied:-0}" \
  --argjson ticketCategoryTotal "${ticket_category_total:-0}" \
  --argjson ticketCategoryRemain "${ticket_category_remain:-0}" \
  --argjson orderRows "${order_rows:-0}" \
  --argjson orderTicketRows "${order_ticket_rows:-0}" \
  --argjson userGuardRows "${user_guard_rows:-0}" \
  --argjson viewerGuardRows "${viewer_guard_rows:-0}" \
  --argjson seatGuardRows "${seat_guard_rows:-0}" \
  '{
    seatAvailable: $seatAvailable,
    seatFrozen: $seatFrozen,
    seatSold: $seatSold,
    seatOccupied: $seatOccupied,
    ticketCategoryTotal: $ticketCategoryTotal,
    ticketCategoryRemain: $ticketCategoryRemain,
    orderRows: $orderRows,
    orderTicketRows: $orderTicketRows,
    userGuardRows: $userGuardRows,
    viewerGuardRows: $viewerGuardRows,
    seatGuardRows: $seatGuardRows
  }' \
  > "${RESULT_DIR}/final_state.json"
