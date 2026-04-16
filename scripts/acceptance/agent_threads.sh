#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/agent_cases.sh"

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
JWT="${JWT:-}"

log() {
  printf '[agent-threads] %s\n' "$*"
}

fail() {
  printf '[agent-threads] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

gateway_curl() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"

  if [[ -n "${payload}" ]]; then
    curl -fsS -X "${method}" "${BASE_URL}${path}" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${JWT}" \
      -d "${payload}"
    return
  fi

  curl -fsS -X "${method}" "${BASE_URL}${path}" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${JWT}"
}

assert_thread_body() {
  local body="$1"
  printf '%s' "${body}" | jq -e '.thread.id != ""' >/dev/null || fail "invalid thread body: ${body}"
}

assert_messages_body() {
  local body="$1"
  printf '%s' "${body}" | jq -e '.messages | type == "array"' >/dev/null || fail "invalid messages body: ${body}"
}

assert_run_body() {
  local body="$1"
  printf '%s' "${body}" | jq -e '.run.id != "" and .run.status != ""' >/dev/null || fail "invalid run body: ${body}"
}

main() {
  require_cmd curl
  require_cmd jq
  [[ -n "${JWT}" ]] || fail "JWT is required"

  log "BASE_URL=${BASE_URL}"

  local create_body
  local thread_id
  local messages_body
  local send_body
  local run_id
  local run_body
  local threads_body

  create_body="$(gateway_curl POST /agent/threads '{}')"
  assert_thread_body "${create_body}"
  thread_id="$(printf '%s' "${create_body}" | jq -r '.thread.id')"
  log "create_thread=${create_body}"

  messages_body="$(gateway_curl GET "/agent/threads/${thread_id}/messages")"
  assert_messages_body "${messages_body}"
  [[ "$(printf '%s' "${messages_body}" | jq '.messages | length')" == "0" ]] || fail "expected empty initial messages"
  log "initial_messages=${messages_body}"

  send_body="$(gateway_curl POST "/agent/threads/${thread_id}/messages" "$(jq -nc --arg message "$(agent_case_order)" '{message:{role:"user",parts:[{type:"text",text:$message}]}}')")"
  printf '%s' "${send_body}" | jq -e '.thread.id != "" and .run.status != "" and (.messages | length) >= 1' >/dev/null \
    || fail "invalid send response: ${send_body}"
  run_id="$(printf '%s' "${send_body}" | jq -r '.run.id')"
  log "send_message=${send_body}"

  run_body="$(gateway_curl GET "/agent/threads/${thread_id}/runs/${run_id}")"
  assert_run_body "${run_body}"
  log "get_run=${run_body}"

  threads_body="$(gateway_curl GET /agent/threads)"
  printf '%s' "${threads_body}" | jq -e '.threads | type == "array"' >/dev/null || fail "invalid threads body: ${threads_body}"
  log "list_threads=${threads_body}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
