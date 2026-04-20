#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/perf/run_rush_rpc_perf.sh"
ANALYZE_SCRIPT_PATH="scripts/perf/analyze_create_order_path.sh"

fail() {
  printf '[rush-rpc-perf-script-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local pattern="$2"

  grep -F -- "${pattern}" "${file}" >/dev/null || fail "missing pattern in ${file}: ${pattern}"
}

assert_not_contains() {
  local file="$1"
  local pattern="$2"

  if grep -F -- "${pattern}" "${file}" >/dev/null; then
    fail "unexpected pattern in ${file}: ${pattern}"
  fi
}

assert_file() {
  local file="$1"
  [[ -f "${file}" ]] || fail "file not found: ${file}"
}

assert_file "${SCRIPT_PATH}"
assert_file "${ANALYZE_SCRIPT_PATH}"

bash -n "${SCRIPT_PATH}"
bash -n "${ANALYZE_SCRIPT_PATH}"

assert_contains "${SCRIPT_PATH}" 'WAIT_FOR_FINAL_STATE="${WAIT_FOR_FINAL_STATE:-1}"'
assert_contains "${SCRIPT_PATH}" 'FINAL_STATE_TIMEOUT_SECONDS="${FINAL_STATE_TIMEOUT_SECONDS:-120}"'
assert_contains "${SCRIPT_PATH}" 'FINAL_STATE_POLL_INTERVAL_SECONDS="${FINAL_STATE_POLL_INTERVAL_SECONDS:-2}"'
assert_contains "${SCRIPT_PATH}" 'final_state_immediate.json'
assert_contains "${SCRIPT_PATH}" 'wait_for_final_state_converged'
assert_not_contains "${SCRIPT_PATH}" 'timing.json'
assert_contains "tests/perf/rush_create_order_rpc.js" "order.OrderRpc/PerfCreateOrder"
assert_not_contains "${ANALYZE_SCRIPT_PATH}" 'timing not found'
assert_not_contains "${ANALYZE_SCRIPT_PATH}" 'timing.json'
assert_not_contains "${ANALYZE_SCRIPT_PATH}" 'client_qps.json'
assert_not_contains "${ANALYZE_SCRIPT_PATH}" 'qpsByClientElapsed'
assert_contains "${ANALYZE_SCRIPT_PATH}" "'^d_order_[0-9]+$'"
assert_contains "${ANALYZE_SCRIPT_PATH}" "'^d_order_ticket_user_[0-9]+$'"
assert_contains "${ANALYZE_SCRIPT_PATH}" "'^d_order_user_guard$'"
assert_contains "${ANALYZE_SCRIPT_PATH}" "'^d_order_viewer_guard$'"
assert_contains "${ANALYZE_SCRIPT_PATH}" "'^d_order_seat_guard$'"

printf '[rush-rpc-perf-script-test] ok\n'
