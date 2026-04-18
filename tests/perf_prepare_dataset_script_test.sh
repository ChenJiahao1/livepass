#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/perf/prepare_rush_perf_dataset.sh"
VERIFY_SCRIPT_PATH="scripts/perf/verify_rush_perf_result.sh"
COMMON_LIB_PATH="scripts/perf/lib/common.sh"
MYSQL_LIB_PATH="scripts/perf/lib/mysql.sh"
HTTP_LIB_PATH="scripts/perf/lib/http.sh"
K6_SCRIPT_PATH="tests/perf/rush_create_order.js"
SUMMARY_LIB_PATH="tests/perf/lib/summary.js"
DATASET_LIB_PATH="tests/perf/lib/dataset.js"

fail() {
  printf '[perf-prepare-dataset-script-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local pattern="$2"

  grep -F -- "${pattern}" "${file}" >/dev/null || fail "missing pattern in ${file}: ${pattern}"
}

assert_file() {
  local file="$1"
  [[ -f "${file}" ]] || fail "file not found: ${file}"
}

assert_file "${SCRIPT_PATH}"
assert_file "${VERIFY_SCRIPT_PATH}"
assert_file "${COMMON_LIB_PATH}"
assert_file "${MYSQL_LIB_PATH}"
assert_file "${HTTP_LIB_PATH}"
assert_file "${K6_SCRIPT_PATH}"
assert_file "${SUMMARY_LIB_PATH}"
assert_file "${DATASET_LIB_PATH}"

bash -n "${SCRIPT_PATH}"
bash -n "${VERIFY_SCRIPT_PATH}"
bash -n "${COMMON_LIB_PATH}"
bash -n "${MYSQL_LIB_PATH}"
bash -n "${HTTP_LIB_PATH}"

assert_contains "${SCRIPT_PATH}" 'USER_COUNT="${USER_COUNT:-5000}"'
assert_contains "${SCRIPT_PATH}" 'SEAT_COUNT="${SEAT_COUNT:-${USER_COUNT}}"'
assert_contains "${SCRIPT_PATH}" 'ROW_COUNT="${ROW_COUNT:-50}"'
assert_contains "${SCRIPT_PATH}" 'COL_COUNT="${COL_COUNT:-100}"'
assert_contains "${SCRIPT_PATH}" 'PERF_SECRET="${PERF_SECRET:-livepass-perf-secret-0001}"'
assert_contains "${SCRIPT_PATH}" 'ticketCount'
assert_contains "${SCRIPT_PATH}" 'purchaseToken'
assert_contains "${SCRIPT_PATH}" 'meta.json'
assert_contains "${SCRIPT_PATH}" 'users.json'
assert_contains "${SCRIPT_PATH}" 'users.csv'
assert_contains "${SCRIPT_PATH}" 'prime_rush_inventory_tmp.sh'
assert_contains "${SCRIPT_PATH}" '/order/purchase/token'
assert_contains "${SCRIPT_PATH}" 'X-LivePass-Perf-Secret'
assert_contains "${SCRIPT_PATH}" 'X-LivePass-Perf-User-Id'

assert_contains "${VERIFY_SCRIPT_PATH}" 'd_order_seat_guard'
assert_contains "${VERIFY_SCRIPT_PATH}" 'remain_number'
assert_contains "${VERIFY_SCRIPT_PATH}" 'seat_status = 3'

assert_contains "${K6_SCRIPT_PATH}" '/order/create'
assert_contains "${K6_SCRIPT_PATH}" '/order/poll'
assert_contains "${K6_SCRIPT_PATH}" 'handleSummary'

assert_contains "${SUMMARY_LIB_PATH}" 'successRate'
assert_contains "${SUMMARY_LIB_PATH}" 'inventoryInsufficientCount'

assert_contains "${DATASET_LIB_PATH}" 'open('
assert_contains "${DATASET_LIB_PATH}" 'purchaseToken'
assert_contains "${DATASET_LIB_PATH}" 'SharedArray'
assert_contains "${DATASET_LIB_PATH}" 'new SharedArray'

printf '[perf-prepare-dataset-script-test] ok\n'
