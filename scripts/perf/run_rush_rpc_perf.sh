#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

source "${ROOT_DIR}/scripts/perf/lib/common.sh"

SHOW_TIME_ID="${SHOW_TIME_ID:-30001}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-40001}"
USER_COUNT="${USER_COUNT:-2000}"
SEAT_COUNT="${SEAT_COUNT:-2000}"
MIN_TICKET_COUNT="${MIN_TICKET_COUNT:-1}"
MAX_TICKET_COUNT="${MAX_TICKET_COUNT:-1}"
VUS="${VUS:-2000}"
ITERATIONS="${ITERATIONS:-2000}"
MAX_DURATION="${MAX_DURATION:-15m}"
ORDER_RPC_TARGET="${ORDER_RPC_TARGET:-127.0.0.1:8082}"
RUN_ID="${RUN_ID:-rush-rpc-${USER_COUNT}-$(date +%Y%m%d%H%M%S)}"
RESULT_DIR="${RESULT_DIR:-${ROOT_DIR}/tmp/perf/results/${RUN_ID}}"
DATASET_DIR="${DATASET_DIR:-${ROOT_DIR}/tmp/perf/${RUN_ID}}"
START_BACKEND_FLAGS="${START_BACKEND_FLAGS:---perf --force-restart --detach --skip-agents}"
ORDER_RPC_ENABLE_CREATE_PERF_LOG="${ORDER_RPC_ENABLE_CREATE_PERF_LOG:-1}"
ORDER_RPC_CREATE_PERF_LOG_SAMPLE_EVERY="${ORDER_RPC_CREATE_PERF_LOG_SAMPLE_EVERY:-1}"
WAIT_FOR_FINAL_STATE="${WAIT_FOR_FINAL_STATE:-1}"
FINAL_STATE_TIMEOUT_SECONDS="${FINAL_STATE_TIMEOUT_SECONDS:-120}"
FINAL_STATE_POLL_INTERVAL_SECONDS="${FINAL_STATE_POLL_INTERVAL_SECONDS:-2}"

final_state_converged() {
  local expected_count="$1"

  jq -e \
    --argjson expectedCount "${expected_count}" \
    '
      .seatFrozen == $expectedCount and
      .seatOccupied == $expectedCount and
      .orderRows == $expectedCount and
      .orderTicketRows == $expectedCount and
      .userGuardRows == $expectedCount and
      .viewerGuardRows == $expectedCount and
      .seatGuardRows == $expectedCount
    ' \
    "${RESULT_DIR}/final_state.json" >/dev/null
}

wait_for_final_state_converged() {
  local expected_count="$1"
  local started_at now

  if [[ "${WAIT_FOR_FINAL_STATE}" != "1" ]]; then
    perf_log "skip final state convergence wait"
    return 0
  fi
  if [[ "${expected_count}" -le 0 ]]; then
    perf_log "skip final state convergence wait because expected count is ${expected_count}"
    return 0
  fi

  started_at="$(date +%s)"
  while true; do
    bash "${ROOT_DIR}/scripts/perf/analyze_create_order_path.sh" "${RESULT_DIR}"
    if final_state_converged "${expected_count}"; then
      perf_log "final state converged to expected count=${expected_count}"
      return 0
    fi

    now="$(date +%s)"
    if (( now - started_at >= FINAL_STATE_TIMEOUT_SECONDS )); then
      perf_log "final state did not converge within ${FINAL_STATE_TIMEOUT_SECONDS}s"
      return 1
    fi

    sleep "${FINAL_STATE_POLL_INTERVAL_SECONDS}"
  done
}

main() {
  perf_require_cmd bash
  perf_require_cmd jq
  perf_require_cmd k6

  perf_ensure_dir "${RESULT_DIR}"
  perf_ensure_dir "${DATASET_DIR}"

  perf_log "rebuild runtime databases"
  bash "${ROOT_DIR}/scripts/deploy/rebuild_databases.sh"

  perf_log "start backend with perf profile"
  ORDER_RPC_ENABLE_CREATE_PERF_LOG="${ORDER_RPC_ENABLE_CREATE_PERF_LOG}" \
  ORDER_RPC_CREATE_PERF_LOG_SAMPLE_EVERY="${ORDER_RPC_CREATE_PERF_LOG_SAMPLE_EVERY}" \
  bash "${ROOT_DIR}/scripts/deploy/start_backend.sh" ${START_BACKEND_FLAGS}

  perf_log "prepare perf dataset"
  SHOW_TIME_ID="${SHOW_TIME_ID}" \
  TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID}" \
  USER_COUNT="${USER_COUNT}" \
  SEAT_COUNT="${SEAT_COUNT}" \
  MIN_TICKET_COUNT="${MIN_TICKET_COUNT}" \
  MAX_TICKET_COUNT="${MAX_TICKET_COUNT}" \
  OUTPUT_DIR="${DATASET_DIR}" \
  bash "${ROOT_DIR}/scripts/perf/prepare_rush_perf_dataset.sh"

  local start_iso start_epoch
  start_iso="$(date --iso-8601=seconds)"
  start_epoch="$(date +%s.%N)"

  perf_log "run gRPC PerfCreateOrder perf"
  (
    local end_iso end_epoch

    cd "${RESULT_DIR}"
    printf 'start_iso=%s\n' "${start_iso}"
    printf 'start_epoch=%s\n' "${start_epoch}"

    DATASET_PATH="${DATASET_DIR}/users.json" \
    ORDER_RPC_TARGET="${ORDER_RPC_TARGET}" \
    VUS="${VUS}" \
    ITERATIONS="${ITERATIONS}" \
    MAX_DURATION="${MAX_DURATION}" \
    k6 run "${ROOT_DIR}/tests/perf/rush_create_order_rpc.js"

    end_iso="$(date --iso-8601=seconds)"
    end_epoch="$(date +%s.%N)"
    printf 'end_iso=%s\n' "${end_iso}"
    printf 'end_epoch=%s\n' "${end_epoch}"

    jq -n \
      --argjson startEpoch "${start_epoch}" \
      --argjson endEpoch "${end_epoch}" \
      '{startEpoch: $startEpoch, endEpoch: $endEpoch, elapsedSeconds: ($endEpoch - $startEpoch)}' \
      > "${RESULT_DIR}/timing.json"
  ) | tee "${RESULT_DIR}/k6.stdout.log"

  bash "${ROOT_DIR}/scripts/perf/analyze_create_order_path.sh" "${RESULT_DIR}"
  cp "${RESULT_DIR}/final_state.json" "${RESULT_DIR}/final_state_immediate.json"

  local create_success_count
  create_success_count="$(jq '.createSuccessCount' "${RESULT_DIR}/summary.json")"
  if ! wait_for_final_state_converged "${create_success_count}"; then
    perf_log "keep latest non-converged final_state.json for inspection"
  fi

  cat > "${RESULT_DIR}/notes.txt" <<EOF
mode=rpc
rpcMethod=PerfCreateOrder
showTimeId=${SHOW_TIME_ID}
ticketCategoryId=${TICKET_CATEGORY_ID}
userCount=${USER_COUNT}
seatCount=${SEAT_COUNT}
minTicketCount=${MIN_TICKET_COUNT}
maxTicketCount=${MAX_TICKET_COUNT}
vus=${VUS}
iterations=${ITERATIONS}
orderRpcTarget=${ORDER_RPC_TARGET}
datasetDir=${DATASET_DIR}
orderRpcPerfLog=${ORDER_RPC_ENABLE_CREATE_PERF_LOG}
orderRpcPerfLogSampleEvery=${ORDER_RPC_CREATE_PERF_LOG_SAMPLE_EVERY}
waitForFinalState=${WAIT_FOR_FINAL_STATE}
finalStateTimeoutSeconds=${FINAL_STATE_TIMEOUT_SECONDS}
finalStatePollIntervalSeconds=${FINAL_STATE_POLL_INTERVAL_SECONDS}
finalStateImmediate=${RESULT_DIR}/final_state_immediate.json
EOF

  perf_log "rpc perf artifacts ready: ${RESULT_DIR}"
}

main "$@"
