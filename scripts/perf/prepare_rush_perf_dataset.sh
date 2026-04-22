#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

source "${ROOT_DIR}/scripts/perf/lib/common.sh"
source "${ROOT_DIR}/scripts/perf/lib/mysql.sh"
source "${ROOT_DIR}/scripts/perf/lib/http.sh"

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
SHOW_TIME_ID="${SHOW_TIME_ID:-30001}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-40001}"
USER_COUNT="${USER_COUNT:-5000}"
SEAT_COUNT="${SEAT_COUNT:-${USER_COUNT}}"
ROW_COUNT="${ROW_COUNT:-50}"
COL_COUNT="${COL_COUNT:-100}"
TICKET_USERS_PER_USER="${TICKET_USERS_PER_USER:-3}"
MIN_TICKET_COUNT="${MIN_TICKET_COUNT:-1}"
MAX_TICKET_COUNT="${MAX_TICKET_COUNT:-3}"
RANDOM_SEED="${RANDOM_SEED:-20260417}"
DATASET_ID="${DATASET_ID:-rush-${SHOW_TIME_ID}-${TICKET_CATEGORY_ID}-${USER_COUNT}-$(date +%Y%m%d%H%M%S)}"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/tmp/perf/${DATASET_ID}}"

PERF_HEADER_NAME="${PERF_HEADER_NAME:-X-LivePass-Perf-Secret}"
PERF_USER_ID_HEADER="${PERF_USER_ID_HEADER:-X-LivePass-Perf-User-Id}"
PERF_SECRET="${PERF_SECRET:-livepass-perf-secret-0001}"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"

MYSQL_DB_USER="${MYSQL_DB_USER:-livepass_user}"
MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-livepass_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-livepass_order}"

USER_PASSWORD="${USER_PASSWORD:-123456}"
MOBILE_PREFIX="${MOBILE_PREFIX:-17$(date +%m%d)}"
SEAT_PRICE="${SEAT_PRICE:-}"
TOKEN_DISTRIBUTION_MODE="${TOKEN_DISTRIBUTION_MODE:-express}"
TOKEN_TAKE_TICKET_MODE="${TOKEN_TAKE_TICKET_MODE:-paper}"

RUN_SUFFIX="$(date +%s)"
USER_ID_BASE="${USER_ID_BASE:-$((7100000000000000 + RUN_SUFFIX * 10000))}"
TICKET_USER_ID_BASE="${TICKET_USER_ID_BASE:-$((7200000000000000 + RUN_SUFFIX * 30000))}"
SEAT_ID_BASE="${SEAT_ID_BASE:-$((7300000000000000 + RUN_SUFFIX * 10000))}"

PROGRAM_ID=""

validate_inputs() {
  perf_require_cmd docker
  perf_require_cmd curl
  perf_require_cmd jq
  perf_require_cmd awk

  [[ "${USER_COUNT}" =~ ^[0-9]+$ ]] || perf_fail "USER_COUNT must be numeric"
  [[ "${SEAT_COUNT}" =~ ^[0-9]+$ ]] || perf_fail "SEAT_COUNT must be numeric"
  [[ "${ROW_COUNT}" =~ ^[0-9]+$ ]] || perf_fail "ROW_COUNT must be numeric"
  [[ "${COL_COUNT}" =~ ^[0-9]+$ ]] || perf_fail "COL_COUNT must be numeric"
  [[ "${TICKET_USERS_PER_USER}" =~ ^[0-9]+$ ]] || perf_fail "TICKET_USERS_PER_USER must be numeric"
  [[ "${MIN_TICKET_COUNT}" =~ ^[0-9]+$ ]] || perf_fail "MIN_TICKET_COUNT must be numeric"
  [[ "${MAX_TICKET_COUNT}" =~ ^[0-9]+$ ]] || perf_fail "MAX_TICKET_COUNT must be numeric"

  (( USER_COUNT > 0 )) || perf_fail "USER_COUNT must be greater than 0"
  (( SEAT_COUNT > 0 )) || perf_fail "SEAT_COUNT must be greater than 0"
  (( ROW_COUNT > 0 )) || perf_fail "ROW_COUNT must be greater than 0"
  (( COL_COUNT > 0 )) || perf_fail "COL_COUNT must be greater than 0"
  (( TICKET_USERS_PER_USER > 0 )) || perf_fail "TICKET_USERS_PER_USER must be greater than 0"
  (( MIN_TICKET_COUNT > 0 )) || perf_fail "MIN_TICKET_COUNT must be greater than 0"
  (( MAX_TICKET_COUNT >= MIN_TICKET_COUNT )) || perf_fail "MAX_TICKET_COUNT must be greater than or equal to MIN_TICKET_COUNT"
  (( MAX_TICKET_COUNT <= TICKET_USERS_PER_USER )) || perf_fail "MAX_TICKET_COUNT must be less than or equal to TICKET_USERS_PER_USER"
  (( ROW_COUNT * COL_COUNT >= SEAT_COUNT )) || perf_fail "ROW_COUNT * COL_COUNT must cover SEAT_COUNT"
}

load_program_context() {
  PROGRAM_ID="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT program_id FROM d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND show_time_id = ${SHOW_TIME_ID} AND status = 1 LIMIT 1")"
  [[ -n "${PROGRAM_ID}" ]] || perf_fail "ticket category not found for showTimeId=${SHOW_TIME_ID} ticketCategoryId=${TICKET_CATEGORY_ID}"

  if [[ -z "${SEAT_PRICE}" ]]; then
    SEAT_PRICE="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT price FROM d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND show_time_id = ${SHOW_TIME_ID} AND status = 1 LIMIT 1")"
  fi

  [[ -n "${SEAT_PRICE}" ]] || perf_fail "failed to resolve seat price"
}

rebuild_users_and_ticket_users() {
  local sql_file
  local ticket_user_total
  local user_id_end
  local ticket_user_id_end

  ticket_user_total=$((USER_COUNT * TICKET_USERS_PER_USER))
  user_id_end=$((USER_ID_BASE + USER_COUNT - 1))
  ticket_user_id_end=$((TICKET_USER_ID_BASE + ticket_user_total - 1))
  sql_file="$(mktemp)"

  cat >"${sql_file}" <<SQL
SET SESSION cte_max_recursion_depth = ${ticket_user_total};

DELETE FROM d_ticket_user
WHERE id BETWEEN ${TICKET_USER_ID_BASE} AND ${ticket_user_id_end};

DELETE FROM d_user
WHERE id BETWEEN ${USER_ID_BASE} AND ${user_id_end};

INSERT INTO d_user (
  id, name, rel_name, mobile, gender, password, email_status, email, rel_authentication_status,
  id_number, address, create_time, edit_time, status
)
WITH RECURSIVE seq AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM seq WHERE n < ${USER_COUNT}
)
SELECT
  ${USER_ID_BASE} + n - 1,
  CONCAT('压测用户-', LPAD(n, 6, '0')),
  CONCAT('压测用户-', LPAD(n, 6, '0')),
  CONCAT('${MOBILE_PREFIX}', LPAD(n, 5, '0')),
  1,
  MD5('${USER_PASSWORD}'),
  0,
  NULL,
  1,
  CONCAT('990101', LPAD(n, 8, '0'), '0001'),
  NULL,
  NOW(),
  NOW(),
  1
FROM seq;

INSERT INTO d_ticket_user (
  id, user_id, rel_name, id_type, id_number, create_time, edit_time, status
)
WITH RECURSIVE seq AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM seq WHERE n < ${ticket_user_total}
)
SELECT
  ${TICKET_USER_ID_BASE} + n - 1,
  ${USER_ID_BASE} + FLOOR((n - 1) / ${TICKET_USERS_PER_USER}),
  CONCAT('压测观演人-', LPAD(FLOOR((n - 1) / ${TICKET_USERS_PER_USER}) + 1, 6, '0'), '-', MOD(n - 1, ${TICKET_USERS_PER_USER}) + 1),
  1,
  CONCAT('880101', LPAD(FLOOR((n - 1) / ${TICKET_USERS_PER_USER}) + 1, 8, '0'), LPAD(MOD(n - 1, ${TICKET_USERS_PER_USER}) + 1, 4, '0')),
  NOW(),
  NOW(),
  1
FROM seq;
SQL

  perf_log "rebuild users and ticket users"
  perf_mysql_import_file "${MYSQL_DB_USER}" "${sql_file}"
  rm -f "${sql_file}"
}

rebuild_seats() {
  local sql_file
  local seat_id_end

  seat_id_end=$((SEAT_ID_BASE + SEAT_COUNT - 1))
  sql_file="$(mktemp)"

  cat >"${sql_file}" <<SQL
SET SESSION cte_max_recursion_depth = ${SEAT_COUNT};

DELETE FROM d_seat
WHERE show_time_id = ${SHOW_TIME_ID};

UPDATE d_ticket_category
SET total_number = ${SEAT_COUNT},
    remain_number = ${SEAT_COUNT},
    edit_time = NOW()
WHERE id = ${TICKET_CATEGORY_ID}
  AND show_time_id = ${SHOW_TIME_ID}
  AND status = 1;

UPDATE d_ticket_category
SET total_number = 0,
    remain_number = 0,
    edit_time = NOW()
WHERE show_time_id = ${SHOW_TIME_ID}
  AND id <> ${TICKET_CATEGORY_ID}
  AND status = 1;

INSERT INTO d_seat (
  id, program_id, show_time_id, ticket_category_id, row_code, col_code, seat_type, price, seat_status,
  freeze_token, freeze_expire_time, create_time, edit_time, status
)
WITH RECURSIVE seq AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM seq WHERE n < ${SEAT_COUNT}
)
SELECT
  ${SEAT_ID_BASE} + n - 1,
  ${PROGRAM_ID},
  ${SHOW_TIME_ID},
  ${TICKET_CATEGORY_ID},
  FLOOR((n - 1) / ${COL_COUNT}) + 1,
  MOD(n - 1, ${COL_COUNT}) + 1,
  1,
  ${SEAT_PRICE},
  1,
  NULL,
  NULL,
  NOW(),
  NOW(),
  1
FROM seq;
SQL

  perf_log "rebuild seats"
  perf_mysql_import_file "${MYSQL_DB_PROGRAM}" "${sql_file}"
  rm -f "${sql_file}"
}

cleanup_order_runtime() {
  local table_name
  local patterns=(
    '^d_order(_[0-9]+)?$'
    '^d_order_ticket_user(_[0-9]+)?$'
    '^d_order_user_guard$'
    '^d_order_viewer_guard$'
    '^d_order_seat_guard$'
  )
  local pattern

  perf_log "cleanup order runtime rows for showTimeId=${SHOW_TIME_ID}"
  for pattern in "${patterns[@]}"; do
    while IFS= read -r table_name; do
      [[ -n "${table_name}" ]] || continue
      perf_mysql_query "${MYSQL_DB_ORDER}" "DELETE FROM \`${table_name}\` WHERE show_time_id = ${SHOW_TIME_ID}" >/dev/null
    done < <(perf_mysql_query "${MYSQL_DB_ORDER}" "SELECT table_name FROM information_schema.tables WHERE table_schema = '${MYSQL_DB_ORDER}' AND table_name REGEXP '${pattern}' ORDER BY table_name")
  done
}

prime_runtime() {
  perf_log "prime rush runtime and seat ledger"
  perf_log "reference helper: scripts/prime_rush_inventory_tmp.sh"
  (
    cd "${ROOT_DIR}"
    go run ./services/order-rpc/cmd/prime_rush_runtime_tmp --programId "${PROGRAM_ID}" --config services/order-rpc/etc/order.yaml
    go run ./services/program-rpc/cmd/prime_program_seat_ledger_tmp --showTimeId "${SHOW_TIME_ID}" --config services/program-rpc/etc/program.yaml
  )
}

issue_purchase_tokens() {
  local jsonl_file="${OUTPUT_DIR}/users.jsonl"
  local csv_file="${OUTPUT_DIR}/users.csv"
  local json_file="${OUTPUT_DIR}/users.json"
  local meta_file="${OUTPUT_DIR}/meta.json"
  local idx
  local user_id
  local ticket_count
  local ticket_user_ids=()
  local ticket_user_ids_csv
  local payload
  local purchase_token
  local slot

  : > "${jsonl_file}"
  printf 'userId,showTimeId,ticketCategoryId,ticketCount,ticketUserIds,purchaseToken\n' > "${csv_file}"

  perf_log "issue purchase tokens"
  for ((idx = 1; idx <= USER_COUNT; idx++)); do
    user_id=$((USER_ID_BASE + idx - 1))
    ticket_count="$(perf_calc_ticket_count "${idx}")"
    ticket_user_ids=()
    for ((slot = 1; slot <= ticket_count; slot++)); do
      ticket_user_ids+=("$((TICKET_USER_ID_BASE + (idx - 1) * TICKET_USERS_PER_USER + slot - 1))")
    done

    ticket_user_ids_csv="$(IFS=,; printf '%s' "${ticket_user_ids[*]}")"
    payload="$(printf '{"showTimeId":%s,"ticketCategoryId":%s,"ticketUserIds":[%s],"distributionMode":"%s","takeTicketMode":"%s"}' \
      "${SHOW_TIME_ID}" \
      "${TICKET_CATEGORY_ID}" \
      "${ticket_user_ids_csv}" \
      "${TOKEN_DISTRIBUTION_MODE}" \
      "${TOKEN_TAKE_TICKET_MODE}")"

    perf_http_post_as_user "/order/purchase/token" "${payload}" "${user_id}"
    [[ "${PERF_HTTP_STATUS}" =~ ^2 ]] || perf_fail "purchase token request failed for userId=${user_id}: http=${PERF_HTTP_STATUS} body=${PERF_HTTP_BODY}"

    purchase_token="$(printf '%s' "${PERF_HTTP_BODY}" | jq -r '.purchaseToken // empty')"
    [[ -n "${purchase_token}" ]] || perf_fail "purchase token missing for userId=${user_id}"

    printf '{"userId":%s,"showTimeId":%s,"ticketCategoryId":%s,"ticketCount":%s,"ticketUserIds":[%s],"purchaseToken":%s}\n' \
      "${user_id}" \
      "${SHOW_TIME_ID}" \
      "${TICKET_CATEGORY_ID}" \
      "${ticket_count}" \
      "${ticket_user_ids_csv}" \
      "$(perf_json_escape "${purchase_token}")" >> "${jsonl_file}"

    printf '%s,%s,%s,%s,"%s","%s"\n' \
      "${user_id}" \
      "${SHOW_TIME_ID}" \
      "${TICKET_CATEGORY_ID}" \
      "${ticket_count}" \
      "${ticket_user_ids_csv}" \
      "${purchase_token}" >> "${csv_file}"
  done

  jq -s '.' "${jsonl_file}" > "${json_file}"

  cat > "${meta_file}" <<JSON
{
  "datasetId": $(perf_json_escape "${DATASET_ID}"),
  "generatedAt": $(perf_json_escape "$(date -Iseconds)"),
  "baseUrl": $(perf_json_escape "${BASE_URL}"),
  "showTimeId": ${SHOW_TIME_ID},
  "ticketCategoryId": ${TICKET_CATEGORY_ID},
  "programId": ${PROGRAM_ID},
  "userCount": ${USER_COUNT},
  "seatCount": ${SEAT_COUNT},
  "rowCount": ${ROW_COUNT},
  "colCount": ${COL_COUNT},
  "ticketUsersPerUser": ${TICKET_USERS_PER_USER},
  "randomSeed": ${RANDOM_SEED},
  "perfHeaderName": $(perf_json_escape "${PERF_HEADER_NAME}"),
  "perfUserIdHeader": $(perf_json_escape "${PERF_USER_ID_HEADER}")
}
JSON

  rm -f "${jsonl_file}"
}

main() {
  validate_inputs
  perf_ensure_dir "${OUTPUT_DIR}"
  load_program_context
  rebuild_users_and_ticket_users
  rebuild_seats
  cleanup_order_runtime
  prime_runtime
  issue_purchase_tokens

  perf_log "dataset prepared"
  printf 'OUTPUT_DIR=%s\nUSERS_JSON=%s\nUSERS_CSV=%s\nMETA_JSON=%s\n' \
    "${OUTPUT_DIR}" \
    "${OUTPUT_DIR}/users.json" \
    "${OUTPUT_DIR}/users.csv" \
    "${OUTPUT_DIR}/meta.json"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
